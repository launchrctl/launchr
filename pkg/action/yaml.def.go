package action

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"

	"gopkg.in/yaml.v3"

	"github.com/launchrctl/launchr/pkg/jsonschema"
	"github.com/launchrctl/launchr/pkg/types"
)

const (
	sErrFieldMustBeArr       = "field must be an array"
	sErrArrElMustBeObj       = "array element must be an object"
	sErrArrEl                = "element must be an array of strings"
	sErrArrOrStrEl           = "element must be an array of strings or a string"
	sErrArrOrMapEl           = "element must be an array of strings or a key-value object"
	sErrEmptyActionImg       = "image field cannot be empty"
	sErrEmptyActionCmd       = "command field cannot be empty"
	sErrEmptyActionArgName   = "action argument name is required"
	sErrEmptyActionOptName   = "action option name is required"
	sErrInvalidActionArgName = "argument name %q is not valid"
	sErrInvalidActionOptName = "option name %q is not valid"
	sErrDupActionVarName     = "argument or option name %q is already defined, a variable name must be unique in the action definition"
)

type errUnsupportedActionVersion struct {
	version string
}

// Error implements error interface.
func (err errUnsupportedActionVersion) Error() string {
	return fmt.Sprintf("unsupported version %q of an action file", err.version)
}

// Is implements errors.Is interface.
func (err errUnsupportedActionVersion) Is(cmp error) bool {
	var errCmp errUnsupportedActionVersion
	ok := errors.As(cmp, &errCmp)
	return ok && errCmp == err
}

var (
	rgxUnescTplRow = regexp.MustCompile(`(?:-|\S+:)(?:\s*)?({{.*}}.*)`)
	rgxTplRow      = regexp.MustCompile(`({{.*}}.*)`)
	rgxVarName     = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_\\-]*$`)
)

// CreateFromYaml creates an action file definition from yaml configuration.
// It returns pointer to Definition or nil on error.
func CreateFromYaml(r io.Reader) (*Definition, error) {
	d := Definition{}
	decoder := yaml.NewDecoder(r)
	err := decoder.Decode(&d)
	if err != nil {
		return nil, err
	}

	// Validate required fields
	switch d.Version {
	case "1":
		if err = validateV1(&d); err != nil {
			return nil, err
		}
	default:
		return nil, errUnsupportedActionVersion{d.Version}
	}
	return &d, nil
}

// CreateFromYamlTpl creates an action file definition from yaml configuration
// as CreateFromYaml but considers that it has unescaped template values.
func CreateFromYamlTpl(b []byte) (*Definition, error) {
	// Find unescaped occurrences of template elements.
	bufRaw := rgxUnescTplRow.ReplaceAllFunc(b, func(match []byte) []byte {
		return rgxTplRow.ReplaceAll(match, []byte(`"$1"`))
	})
	r := bytes.NewReader(bufRaw)
	return CreateFromYaml(r)
}

// Definition is a representation of an action file
type Definition struct {
	Version string     `yaml:"version"`
	WD      string     `yaml:"working_directory"`
	Action  *DefAction `yaml:"action"`
}

// Content implements Loader interface.
func (d *Definition) Content() ([]byte, error) {
	w := &bytes.Buffer{}
	err := yaml.NewEncoder(w).Encode(d)
	return w.Bytes(), err
}

// Load implements Loader interface.
func (d *Definition) Load(_ LoadContext) (*Definition, error) {
	return d.LoadRaw()
}

// LoadRaw implements Loader interface.
func (d *Definition) LoadRaw() (*Definition, error) {
	return d, nil
}

var yamlTree = newGlobalYamlParseMeta()

// UnmarshalYAML implements yaml.Unmarshaler to parse action definition.
func (d *Definition) UnmarshalYAML(node *yaml.Node) (err error) {
	type yamlDef Definition
	var yd yamlDef
	yamlTree.addDef(d, node)
	defer yamlTree.removeDef(d)
	if err = node.Decode(&yd); err != nil {
		return err
	}
	*d = Definition(yd)
	// Set default version to 1
	if d.Version == "" {
		d.Version = "1"
	}
	return nil
}

func validateV1(_ *Definition) error {
	// The schema is validated on parsing.
	return nil
}

// DefAction holds action configuration
type DefAction struct {
	Title       string                 `yaml:"title"`
	Description string                 `yaml:"description"`
	Aliases     []string               `yaml:"alias"`
	Arguments   ArgumentsList          `yaml:"arguments"`
	Options     OptionsList            `yaml:"options"`
	Command     StrSliceOrStr          `yaml:"command"`
	Image       string                 `yaml:"image"`
	Build       *types.BuildDefinition `yaml:"build"`
	ExtraHosts  StrSlice               `yaml:"extra_hosts"`
	Env         EnvSlice               `yaml:"env"`
	User        string                 `yaml:"user"`
}

// UnmarshalYAML implements yaml.Unmarshaler to parse action definition.
func (a *DefAction) UnmarshalYAML(n *yaml.Node) (err error) {
	type yamlT DefAction
	var y yamlT
	if err = n.Decode(&y); err != nil {
		return err
	}
	*a = DefAction(y)

	if a.Image == "" {
		l, c := yamlNodeLineCol(n, "image")
		return yamlTypeErrorLine(sErrEmptyActionImg, l, c)
	}
	if len(a.Command) == 0 {
		l, c := yamlNodeLineCol(n, "command")
		return yamlTypeErrorLine(sErrEmptyActionCmd, l, c)
	}
	return nil
}

// StrSlice is an array of strings for command execution.
type StrSlice []string

// UnmarshalYAML implements yaml.Unmarshaler to parse a string or a list of strings.
func (l *StrSlice) UnmarshalYAML(n *yaml.Node) (err error) {
	if n.Kind == yaml.ScalarNode {
		return yamlTypeErrorLine(sErrArrEl, n.Line, n.Column)
	}
	var s StrSliceOrStr
	err = n.Decode(&s)
	if err != nil {
		return err
	}
	*l = StrSlice(s)
	return err
}

// StrSliceOrStr is an array of strings for command execution.
type StrSliceOrStr []string

// UnmarshalYAML implements yaml.Unmarshaler to parse a string or a list of strings.
func (l *StrSliceOrStr) UnmarshalYAML(n *yaml.Node) (err error) {
	type yamlT StrSliceOrStr
	if n.Kind == yaml.ScalarNode {
		var s string
		err = n.Decode(&s)
		*l = StrSliceOrStr{s}
		return err
	}
	var s yamlT
	err = n.Decode(&s)
	if err != nil {
		return yamlTypeErrorLine(sErrArrOrStrEl, n.Line, n.Column)
	}
	*l = StrSliceOrStr(s)
	return err
}

// EnvSlice is an array of env vars or key-value.
type EnvSlice []string

// UnmarshalYAML implements yaml.Unmarshaler to parse env []string or map[string]string.
func (l *EnvSlice) UnmarshalYAML(n *yaml.Node) (err error) {
	if n.Kind == yaml.MappingNode {
		var m map[string]string
		err = n.Decode(&m)
		if err != nil {
			return yamlTypeErrorLine(sErrArrOrMapEl, n.Line, n.Column)
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
			return yamlTypeErrorLine(sErrArrOrMapEl, n.Line, n.Column)
		}
		*l = s
		return err
	}

	return yamlTypeErrorLine(sErrArrOrMapEl, n.Line, n.Column)
}

// ArgumentsList is used for custom yaml parsing of arguments list.
type ArgumentsList []*Argument

// UnmarshalYAML implements yaml.Unmarshaler to parse for ArgumentsList.
func (l *ArgumentsList) UnmarshalYAML(nodeList *yaml.Node) (err error) {
	*l, err = unmarshalListYaml[*Argument](nodeList)
	return err
}

// Argument stores command arguments declaration.
type Argument struct {
	Name        string            `yaml:"name"`
	Title       string            `yaml:"title"`
	Description string            `yaml:"description"`
	Type        jsonschema.Type   `yaml:"type"`
	Process     []ValueProcessDef `yaml:"process"`
	RawMap      map[string]interface{}
}

// UnmarshalYAML implements yaml.Unmarshaler to parse Argument.
func (a *Argument) UnmarshalYAML(node *yaml.Node) (err error) {
	type yamlT Argument
	var y yamlT
	errStr := []string{sErrEmptyActionArgName, sErrInvalidActionArgName, sErrDupActionVarName}
	if err = unmarshalVarYaml(node, &y, errStr); err != nil {
		return err
	}
	*a = Argument(y)
	return nil
}

// OptionsList is used for custom yaml parsing of options list.
type OptionsList []*Option

// UnmarshalYAML implements yaml.Unmarshaler to parse OptionsList.
func (l *OptionsList) UnmarshalYAML(nodeList *yaml.Node) (err error) {
	*l, err = unmarshalListYaml[*Option](nodeList)
	return err
}

// Option stores command options declaration.
type Option struct {
	Name        string            `yaml:"name"`
	Shorthand   string            `yaml:"shorthand"` // @todo test definition, validate, catch panic if overlay, add to readme.
	Title       string            `yaml:"title"`
	Description string            `yaml:"description"`
	Type        jsonschema.Type   `yaml:"type"`
	Default     interface{}       `yaml:"default"`
	Required    bool              `yaml:"required"` // @todo that conflicts with json schema object definition
	Process     []ValueProcessDef `yaml:"process"`
	RawMap      map[string]interface{}
}

// ValueProcessDef stores information about processor and options that should be applied to processor.
type ValueProcessDef struct {
	Processor string                 `yaml:"processor"`
	Options   map[string]interface{} `yaml:"options"`
}

// UnmarshalYAML implements yaml.Unmarshaler to parse Option.
func (o *Option) UnmarshalYAML(node *yaml.Node) (err error) {
	type yamlT Option
	var y yamlT
	errStr := []string{sErrEmptyActionOptName, sErrInvalidActionOptName, sErrDupActionVarName}
	if err = unmarshalVarYaml(node, &y, errStr); err != nil {
		return err
	}
	*o = Option(y)
	dval := getDefaultByType(o)
	if errDef, ok := dval.(error); ok {
		return yamlTypeErrorLine(errDef.Error(), node.Line, node.Column)
	}
	o.Default = dval
	o.RawMap["default"] = o.Default
	return nil
}

func unmarshalVarYaml(n *yaml.Node, v any, errStr []string) (err error) {
	if err = n.Decode(v); err != nil {
		return err
	}
	vname := reflectValRef(v, "Name").(*string)
	vtype := reflectValRef(v, "Type").(*jsonschema.Type)
	vtitle := reflectValRef(v, "Title").(*string)
	vraw := reflectValRef(v, "RawMap").(*map[string]interface{})

	if *vname == "" {
		return yamlTypeErrorLine(errStr[0], n.Line, n.Column)
	}
	if !rgxVarName.MatchString(*vname) {
		l, c := yamlNodeLineCol(n, "name")
		return yamlTypeErrorLine(fmt.Sprintf(errStr[1], *vname), l, c)
	}
	dups := yamlTree.dupsByNode(n)
	if !dups.isUnique(*vname) {
		l, c := yamlNodeLineCol(n, "name")
		return yamlTypeErrorLine(fmt.Sprintf(errStr[2], *vname), l, c)
	}
	if err = n.Decode(vraw); err != nil {
		return err
	}
	if *vtype == "" {
		*vtype = jsonschema.String
	}
	if *vtitle == "" {
		*vtitle = *vname
	}
	(*vraw)["type"] = *vtype
	// @todo review hardcoded array elements types when array is properly implemented.
	if *vtype == jsonschema.Array {
		items, ok := (*vraw)["items"].(map[string]interface{})
		if !ok {
			items = map[string]interface{}{}
		}

		items["type"] = jsonschema.String

		// Override if enum is specified
		if enum, ok := items["enum"]; ok {
			items["enum"] = enum
		}

		(*vraw)["items"] = items
	}

	return nil
}

func unmarshalListYaml[T any](nl *yaml.Node) ([]T, error) {
	if nl.Kind != yaml.SequenceNode {
		return nil, yamlTypeErrorLine(sErrFieldMustBeArr, nl.Line, nl.Column)
	}
	l := make([]T, 0, len(nl.Content))
	var errs *yaml.TypeError
	for _, node := range nl.Content {
		if node.Kind != yaml.MappingNode {
			errs = yamlMergeErrors(errs, yamlTypeErrorLine(sErrArrElMustBeObj, node.Line, node.Column))
			continue
		}
		var v T
		if err := node.Decode(&v); err != nil {
			if errType, ok := err.(*yaml.TypeError); ok {
				errs = yamlMergeErrors(errs, errType)
				continue
			}
			return nil, err
		}
		l = append(l, v)
	}
	if errs != nil {
		return l, errs
	}

	return l, nil
}
