package action

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"reflect"
	"regexp"

	"gopkg.in/yaml.v3"
)

var (
	errFieldMustBeArr = errors.New("field must be an array")
	errArrElMustBeObj = errors.New("array element must be an object")
	errArrOrStrEl     = errors.New("element must be a string array or a string")
	errArrOrMapEl     = errors.New("element must be a string array or a key-value object")
)

var (
	rgxUnescapedTplRow = regexp.MustCompile(`(?:-|\S+:)(?:\s*)?({{.*}}.*)`)
	rgxTplRow          = regexp.MustCompile(`({{.*}}.*)`)
)

// CreateFromYamlTpl creates action definition from yaml configuration
// as CreateFromYaml but considers that it has unescaped template values.
func CreateFromYamlTpl(b []byte) (*Action, error) {
	// Find unescaped occurrences of template elements.
	bufRaw := rgxUnescapedTplRow.ReplaceAllFunc(b, func(match []byte) []byte {
		return rgxTplRow.ReplaceAll(match, []byte("\"$1\""))
	})
	r := bytes.NewReader(bufRaw)
	return CreateFromYaml(r)
}

// CreateFromYaml creates action definition from yaml configuration.
// It returns pointer to Action or nil on error.
func CreateFromYaml(r io.Reader) (*Action, error) {
	cfg := Action{}
	decoder := yaml.NewDecoder(r)
	err := decoder.Decode(&cfg)
	if err != nil {
		return nil, err
	}

	cfg.initDefaults()

	// Validate required fields
	switch cfg.Version {
	case "1":
		if err := validateV1(&cfg); err != nil {
			return nil, err
		}
	default:
		return nil, errUnsupportedActionVersion{cfg.Version}
	}
	return &cfg, nil
}

func unmarshalListYaml[T *Argument | *Option](nodeList *yaml.Node) ([]T, error) {
	if nodeList.Kind != yaml.SequenceNode {
		return nil, errFieldMustBeArr
	}
	args := make([]T, 0, len(nodeList.Content))
	for _, node := range nodeList.Content {
		if node.Kind != yaml.MappingNode {
			return nil, errArrElMustBeObj
		}
		var arg T
		err := node.Decode(&arg)
		if err != nil {
			return nil, err
		}

		var raw map[string]interface{}
		err = node.Decode(&raw)
		if err != nil {
			return nil, err
		}
		// Set Raw data.
		refArg := reflect.ValueOf(arg).Elem()
		refArg.FieldByName("RawMap").Set(reflect.ValueOf(raw))
		args = append(args, arg)
	}

	return args, nil
}

// UnmarshalYAML implements yaml.Unmarshaler to parse for ArgumentsList.
func (l *ArgumentsList) UnmarshalYAML(nodeList *yaml.Node) (err error) {
	*l, err = unmarshalListYaml[*Argument](nodeList)
	return err
}

// UnmarshalYAML implements yaml.Unmarshaler to parse for OptionsList.
func (l *OptionsList) UnmarshalYAML(nodeList *yaml.Node) (err error) {
	*l, err = unmarshalListYaml[*Option](nodeList)
	return err
}

type yamlStrSlice StrSlice

// UnmarshalYAML implements yaml.Unmarshaler to parse a string or a list of strings.
func (l *StrSlice) UnmarshalYAML(n *yaml.Node) (err error) {
	if n.Kind == yaml.ScalarNode {
		var s string
		err = n.Decode(&s)
		*l = StrSlice{s}
		return err
	}
	var s yamlStrSlice
	err = n.Decode(&s)
	if err != nil {
		return errArrOrStrEl
	}
	*l = StrSlice(s)
	return err
}

// UnmarshalYAML implements yaml.Unmarshaler to parse env []string or map[string]string.
func (l *EnvSlice) UnmarshalYAML(n *yaml.Node) (err error) {
	if n.Kind == yaml.MappingNode {
		var m map[string]string
		err = n.Decode(&m)
		if err != nil {
			return errArrOrMapEl
		}
		newl := make(EnvSlice, len(m))
		i := 0
		for k, v := range m {
			newl[i] = k + "=" + v
			i++
		}
		*l = newl
		return err
	}
	if n.Kind == yaml.SequenceNode {
		var s []string
		err = n.Decode(&s)
		if err != nil {
			return errArrOrMapEl
		}
		*l = s
		return err
	}

	// @todo Set line and column to the error message.
	return errArrOrMapEl
}

type yamlFileLoader struct {
	processor LoadProcessor
	raw       *Action
	cached    []byte
	open      func() (fs.File, error)
}

func (l *yamlFileLoader) Content() ([]byte, error) {
	// @todo unload unused.
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

func (l *yamlFileLoader) LoadRaw() (*Action, error) {
	var err error
	buf, err := l.Content()
	if err != nil {
		return nil, err
	}
	if l.raw == nil {
		l.raw, err = CreateFromYamlTpl(buf)
		if err != nil {
			return nil, err
		}
	}
	return l.raw, err
}

func (l *yamlFileLoader) Load() (res *Action, err error) {
	// Open file and save content for future.
	c, err := l.Content()
	if err != nil {
		return nil, err
	}
	buf := make([]byte, len(c))
	copy(buf, c)
	buf, err = l.processor.Process(buf)
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
