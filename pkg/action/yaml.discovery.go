package action

import (
	"bufio"
	"bytes"
	"io"
	"io/fs"
	"regexp"
	"sync"
)

var rgxYamlFile = regexp.MustCompile(`^action\.(yaml|yml)$`)

// NewYamlDiscovery is an implementation of discovery for searching yaml files.
func NewYamlDiscovery(fs fs.FS) *Discovery {
	return NewDiscovery(fs, YamlDiscoveryStrategy{TargetRgx: rgxYamlFile})
}

// YamlDiscoveryStrategy is a yaml discovery strategy.
type YamlDiscoveryStrategy struct {
	TargetRgx *regexp.Regexp
}

// IsValid implements DiscoveryStrategy.
func (y YamlDiscoveryStrategy) IsValid(name string) bool {
	return y.TargetRgx.MatchString(name)
}

// Loader implements DiscoveryStrategy.
func (y YamlDiscoveryStrategy) Loader(l FileLoadFn, p ...LoadProcessor) Loader {
	return &yamlFileLoader{
		open: l,
		processor: NewPipeProcessor(
			append([]LoadProcessor{escapeYamlTplCommentsProcessor{}}, p...)...,
		),
	}
}

type yamlFileLoader struct {
	processor LoadProcessor
	raw       *Definition
	cached    []byte
	open      func() (fs.File, error)
	mx        sync.Mutex
}

func (l *yamlFileLoader) Content() ([]byte, error) {
	l.mx.Lock()
	defer l.mx.Unlock()
	// @todo unload unused, maybe manager must do it.
	var err error
	if l.cached != nil {
		return l.cached, nil
	}
	f, err := l.open()
	if err != nil {
		return nil, err
	}
	defer f.Close()
	l.cached, err = io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	return l.cached, nil
}

func (l *yamlFileLoader) LoadRaw() (*Definition, error) {
	var err error
	buf, err := l.Content()
	if err != nil {
		return nil, err
	}
	l.mx.Lock()
	defer l.mx.Unlock()
	if l.raw == nil {
		l.raw, err = CreateFromYamlTpl(buf)
		if err != nil {
			return nil, err
		}
	}
	return l.raw, err
}

func (l *yamlFileLoader) Load(ctx LoadContext) (res *Definition, err error) {
	// Open a file and cache content for future reads.
	c, err := l.Content()
	if err != nil {
		return nil, err
	}
	buf := make([]byte, len(c))
	copy(buf, c)
	buf, err = l.processor.Process(ctx, buf)
	if err != nil {
		return nil, err
	}
	r := bytes.NewReader(buf)
	res, err = CreateFromYaml(r)
	if err != nil {
		return nil, err
	}
	return res, err
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
