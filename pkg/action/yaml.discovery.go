package action

import (
	"bufio"
	"bytes"
	"io"
	"regexp"
	"sync"
)

var (
	// rgxYamlFilepath is a regex for a yaml path with unix and windows support.
	rgxYamlFilepath = regexp.MustCompile(`(^actions|.*[\\/]actions)[\\/][^\\/]+[\\/]action\.y(a)?ml$`)
	// rgxYamlRootFile is a regex for a yaml file located in root dir only.
	rgxYamlRootFile = regexp.MustCompile(`^action\.y(a)?ml$`)
)

// NewYamlDiscovery is an implementation of discovery for searching yaml files.
func NewYamlDiscovery(fs DiscoveryFS) *Discovery {
	return NewDiscovery(fs, YamlDiscoveryStrategy{TargetRgx: rgxYamlFilepath})
}

// YamlDiscoveryStrategy is a yaml discovery strategy.
type YamlDiscoveryStrategy struct {
	TargetRgx *regexp.Regexp
	AllowRoot bool
}

// IsValid implements [DiscoveryStrategy].
func (y YamlDiscoveryStrategy) IsValid(path string) bool {
	return y.TargetRgx.MatchString(path)
}

// Loader implements [DiscoveryStrategy].
func (y YamlDiscoveryStrategy) Loader(l FileLoadFn, p ...LoadProcessor) Loader {
	return &YamlFileLoader{
		YamlLoader: YamlLoader{
			Processor: NewPipeProcessor(
				append([]LoadProcessor{escapeYamlTplCommentsProcessor{}}, p...)...,
			),
		},
		FileOpen: l,
	}
}

// YamlLoader loads action yaml from a string.
type YamlLoader struct {
	Bytes     []byte        // Bytes represents yaml content bytes.
	Processor LoadProcessor // Processor processes variables inside the file.

	raw *Definition // raw holds unprocessed definition.
	mx  sync.Mutex  // mx is a mutex for loading and processing only once.
}

// Content implements [Loader] interface.
func (l *YamlLoader) Content() ([]byte, error) {
	return l.Bytes, nil
}

// LoadRaw implements [Loader] interface.
func (l *YamlLoader) LoadRaw() (*Definition, error) {
	var err error
	buf, err := l.Content()
	if err != nil {
		return nil, err
	}
	l.mx.Lock()
	defer l.mx.Unlock()
	if l.raw == nil {
		l.raw, err = NewDefFromYamlTpl(buf)
		if err != nil {
			return nil, err
		}
	}
	return l.raw, err
}

// Load implements [Loader] interface.
func (l *YamlLoader) Load(ctx LoadContext) (res *Definition, err error) {
	// Open a file and cache content for future reads.
	c, err := l.Content()
	if err != nil {
		return nil, err
	}
	buf := make([]byte, len(c))
	copy(buf, c)
	if l.Processor != nil {
		buf, err = l.Processor.Process(ctx, buf)
		if err != nil {
			return nil, err
		}
	}
	res, err = NewDefFromYaml(buf)
	if err != nil {
		return nil, err
	}
	return res, err
}

// YamlFileLoader loads action yaml from a file.
type YamlFileLoader struct {
	YamlLoader
	FileOpen FileLoadFn // FileOpen lazy loads the content of the file.
}

// LoadRaw implements [Loader] interface.
func (l *YamlFileLoader) LoadRaw() (*Definition, error) {
	_, err := l.Content()
	if err != nil {
		return nil, err
	}
	return l.YamlLoader.LoadRaw()
}

// Load implements [Loader] interface.
func (l *YamlFileLoader) Load(ctx LoadContext) (res *Definition, err error) {
	// Open a file and cache content for future reads.
	_, err = l.Content()
	if err != nil {
		return nil, err
	}
	return l.YamlLoader.Load(ctx)
}

// Content implements [Loader] interface.
func (l *YamlFileLoader) Content() ([]byte, error) {
	l.mx.Lock()
	defer l.mx.Unlock()
	// @todo unload unused, maybe manager must do it.
	var err error
	if l.Bytes != nil {
		return l.Bytes, nil
	}
	f, err := l.FileOpen()
	if err != nil {
		return nil, err
	}
	defer f.Close()
	l.Bytes, err = io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	return l.Bytes, nil
}

type escapeYamlTplCommentsProcessor struct{}

func (p escapeYamlTplCommentsProcessor) Process(_ LoadContext, b []byte) ([]byte, error) {
	// Read by line.
	scanner := bufio.NewScanner(bytes.NewBuffer(b))
	res := make([]byte, 0, len(b))
	for scanner.Scan() {
		l := scanner.Bytes()
		if i := bytes.IndexByte(l, '#'); i != -1 {
			// Check the comment symbol is not inside a string.
			// Multiline strings are not supported for now.
			if !(bytes.LastIndexByte(l[:i], '"') != -1 && bytes.IndexByte(l[i:], '"') != -1 ||
				bytes.LastIndexByte(l[:i], '\'') != -1 && bytes.IndexByte(l[i:], '\'') != -1) {
				// Strip data after comment symbol.
				l = l[:i]
			}
		}
		// Collect the modified lines.
		res = append(res, l...)
		res = append(res, '\n')
	}
	return res, nil
}
