package action

import (
	"io"
	"path/filepath"
	"regexp"
	"sync"
)

var (
	// rgxYamlFilepath is a regex for a yaml path with unix and windows support.
	rgxYamlFilepath = regexp.MustCompile(`^(actions|[^\s!<>:"|?*]+/actions)/[^\s!<>:"|?*/]+/action\.y(a)?ml$`)
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
	return y.TargetRgx.MatchString(filepath.ToSlash(path))
}

// Loader implements [DiscoveryStrategy].
func (y YamlDiscoveryStrategy) Loader(l FileLoadFn, p ...LoadProcessor) Loader {
	return &YamlFileLoader{
		YamlLoader: YamlLoader{
			Processor: NewPipeProcessor(p...),
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

func (l *YamlLoader) loadRaw() (*Definition, error) {
	var err error
	buf, err := l.Content()
	if err != nil {
		return nil, err
	}
	l.mx.Lock()
	defer l.mx.Unlock()
	if l.raw == nil {
		l.raw, err = NewDefFromYaml(buf)
		if err != nil {
			return nil, err
		}
	}
	return l.raw, err
}

// Load implements [Loader] interface.
func (l *YamlLoader) Load(ctx *LoadContext) (res *Definition, err error) {
	// Open a file and cache content for future reads.
	c, err := l.Content()
	if err != nil {
		return nil, err
	}
	if ctx == nil {
		return l.loadRaw()
	}
	res, err = NewDefFromYaml(c)
	if l.Processor != nil {
		err = processStructStringsInPlace(res, func(s string) (string, error) {
			return l.Processor.Process(ctx, s)
		})
		if err != nil {
			return nil, err
		}
	}
	return res, err
}

// YamlFileLoader loads action yaml from a file.
type YamlFileLoader struct {
	YamlLoader
	FileOpen FileLoadFn // FileOpen lazy loads the content of the file.
}

// Load implements [Loader] interface.
func (l *YamlFileLoader) Load(ctx *LoadContext) (res *Definition, err error) {
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
