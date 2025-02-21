package action

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"

	"gopkg.in/yaml.v3"

	"github.com/launchrctl/launchr/pkg/driver"
	"github.com/launchrctl/launchr/pkg/jsonschema"
)

const (
	sErrFieldMustBeArr = "field must be an array"
	sErrArrElMustBeObj = "array element must be an object"
	sErrArrEl          = "element must be an array of strings"
	sErrArrOrStrEl     = "element must be an array of strings or a string"
	sErrArrOrMapEl     = "element must be an array of strings or a key-value object"

	sErrEmptyRuntimeImg        = "image field cannot be empty"
	sErrEmptyRuntimeCmd        = "command field cannot be empty"
	sErrEmptyActionParamName   = "parameter name is required"
	sErrInvalidActionParamName = "parameter name %q is not valid"
	sErrDupActionParamName     = "parameter name %q is already defined, a variable name must be unique in the action definition"
	sErrActionDefMissing       = "action definition is missing in the declaration"
	sErrEmptyProcessorID       = "invalid configuration, processor ID is required"

	// Runtime types.
	runtimeTypePlugin    DefRuntimeType = "plugin"
	runtimeTypeContainer DefRuntimeType = "container"
)

type errUnsupportedActionVersion struct {
	version string
}

// Error implements error interface.
func (err errUnsupportedActionVersion) Error() string {
	return fmt.Sprintf("unsupported version %q of an action file", err.version)
}

// Is implements [errors.Is] interface.
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

// NewDefFromYaml creates an action file definition from yaml configuration.
// It returns pointer to [Definition] or nil on error.
func NewDefFromYaml(b []byte) (*Definition, error) {
	d := Definition{}
	r := bytes.NewReader(b)
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

// NewDefFromYamlTpl creates an action file definition from yaml configuration
// as [NewDefFromYaml] but considers that it has unescaped template values.
func NewDefFromYamlTpl(b []byte) (*Definition, error) {
	// Find unescaped occurrences of template elements.
	bufRaw := rgxUnescTplRow.ReplaceAllFunc(b, func(match []byte) []byte {
		return rgxTplRow.ReplaceAll(match, []byte(`"$1"`))
	})
	return NewDefFromYaml(bufRaw)
}

// Definition is a representation of an action file.
type Definition struct {
	Version string      `yaml:"version"`
	WD      string      `yaml:"working_directory"`
	Action  *DefAction  `yaml:"action"`
	Runtime *DefRuntime `yaml:"runtime"`
}

// Content implements [Loader] interface.
func (d *Definition) Content() ([]byte, error) {
	w := &bytes.Buffer{}
	err := yaml.NewEncoder(w).Encode(d)
	return w.Bytes(), err
}

// Load implements [Loader] interface.
func (d *Definition) Load(_ LoadContext) (*Definition, error) {
	return d.LoadRaw()
}

// LoadRaw implements [Loader] interface.
func (d *Definition) LoadRaw() (*Definition, error) {
	return d, nil
}

var yamlTree = newGlobalYamlParseMeta()

// UnmarshalYAML implements [yaml.Unmarshaler] to parse action definition.
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
	if d.Runtime == nil {
		return yamlTypeErrorLine("missing runtime configuration", node.Line, node.Column)
	}
	return nil
}

func validateV1(d *Definition) error {
	// The schema is validated on parsing.
	if d.Action == nil {
		return errors.New(sErrActionDefMissing)
	}
	return nil
}

// DefAction holds action configuration.
type DefAction struct {
	Title       string         `yaml:"title"`
	Description string         `yaml:"description"`
	Aliases     []string       `yaml:"alias"`
	Arguments   ParametersList `yaml:"arguments"`
	Options     ParametersList `yaml:"options"`
}

// UnmarshalYAML implements [yaml.Unmarshaler] to parse action definition.
func (a *DefAction) UnmarshalYAML(n *yaml.Node) (err error) {
	type yamlT DefAction
	var y yamlT
	if err = n.Decode(&y); err != nil {
		return err
	}
	*a = DefAction(y)
	return nil
}

// DefRuntimeType is a runtime type.
type DefRuntimeType string

// UnmarshalYAML implements [yaml.Unmarshaler] to parse runtime type.
func (r *DefRuntimeType) UnmarshalYAML(n *yaml.Node) (err error) {
	var s string
	if err = n.Decode(&s); err != nil {
		return err
	}
	*r = DefRuntimeType(s)
	switch *r {
	case runtimeTypePlugin, runtimeTypeContainer:
		return nil
	case "":
		return yamlTypeErrorLine("empty runtime type", n.Line, n.Column)
	default:
		return yamlTypeErrorLine(fmt.Sprintf("unknown runtime type %q", *r), n.Line, n.Column)
	}
}

// DefRuntimeContainer has container-specific runtime configuration.
type DefRuntimeContainer struct {
	Command    StrSliceOrStr           `yaml:"command"`
	Image      string                  `yaml:"image"`
	Build      *driver.BuildDefinition `yaml:"build"`
	ExtraHosts StrSlice                `yaml:"extra_hosts"`
	Env        EnvSlice                `yaml:"env"`
	User       string                  `yaml:"user"`
}

// UnmarshalYAML implements [yaml.Unmarshaler] to parse runtime container definition.
func (r *DefRuntimeContainer) UnmarshalYAML(n *yaml.Node) (err error) {
	type yamlT DefRuntimeContainer
	var y yamlT
	if err = n.Decode(&y); err != nil {
		return err
	}
	*r = DefRuntimeContainer(y)
	if r.Image == "" {
		l, c := yamlNodeLineCol(n, "image")
		return yamlTypeErrorLine(sErrEmptyRuntimeImg, l, c)
	}
	if len(r.Command) == 0 {
		l, c := yamlNodeLineCol(n, "command")
		return yamlTypeErrorLine(sErrEmptyRuntimeCmd, l, c)
	}
	return err
}

// DefRuntime contains action runtime configuration.
type DefRuntime struct {
	Type      DefRuntimeType `yaml:"type"`
	Container *DefRuntimeContainer
}

// UnmarshalYAML implements [yaml.Unmarshaler] to parse runtime definition.
func (r *DefRuntime) UnmarshalYAML(n *yaml.Node) (err error) {
	// If node was defined as a string, example "plugin"
	var rtype DefRuntimeType
	if n.Kind == yaml.ScalarNode {
		err = n.Decode(&rtype)
		r.Type = rtype
		if r.Type != runtimeTypePlugin {
			return yamlTypeErrorLine("missing runtime configuration", n.Line, n.Column)
		}
		return err
	}

	// Preparse type to proceed parsing.
	ntype := yamlFindNodeByKey(n, "type")
	if ntype == nil {
		return yamlTypeErrorLine("missing runtime type definition", n.Line, n.Column)
	}
	if err = ntype.Decode(&rtype); err != nil {
		return err
	}

	// Parse runtime configuration.
	r.Type = rtype
	switch r.Type {
	case runtimeTypePlugin:
		return nil
	case runtimeTypeContainer:
		err = n.Decode(&r.Container)
		return err
	default:
		// Error is already returned on runtime type parsing.
		panic(fmt.Sprintf("runtime type not implemented: %s", r.Type))
	}
}

// StrSlice is an array of strings for command execution.
type StrSlice []string

// UnmarshalYAML implements [yaml.Unmarshaler] to parse a string or a list of strings.
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

// UnmarshalYAML implements [yaml.Unmarshaler] to parse a string or a list of strings.
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

// EnvSlice is an array of runtime vars or key-value.
type EnvSlice []string

// UnmarshalYAML implements [yaml.Unmarshaler] to parse runtime []string or map[string]string.
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

// ParametersList is used for custom yaml parsing of arguments list.
type ParametersList []*DefParameter

// UnmarshalYAML implements [yaml.Unmarshaler] to parse for [ParametersList].
func (l *ParametersList) UnmarshalYAML(nodeList *yaml.Node) (err error) {
	*l, err = unmarshalParamListYaml(nodeList)
	return err
}

// DefParameter stores command argument or option declaration.
type DefParameter struct {
	Title       string          `yaml:"title"`
	Description string          `yaml:"description"`
	Type        jsonschema.Type `yaml:"type"`
	Default     any             `yaml:"default"`
	Enum        []any           `yaml:"enum"`
	Items       *DefArrayItems  `yaml:"items"`

	// Action specific behavior for parameters.
	// Name is an action unique parameter name used.
	Name string `yaml:"name"`
	// Shorthand is a short name 1 syllable name used in Console.
	// @todo test definition, validate, catch panic if overlay, add to readme.
	Shorthand string `yaml:"shorthand"`
	// Required indicates if the parameter is mandatory.
	// It's not correct json schema, and it's processed to a correct place later.
	Required bool `yaml:"required"`
	// Process is an array of [ValueProcessor] to a value.
	Process []DefValueProcessor `yaml:"process"`
	// processors is an instantiated list of processor handlers.
	processors []ValueProcessorHandler
	// raw is a raw parameter declaration to support all JSON Schema features.
	raw map[string]any
}

// UnmarshalYAML implements [yaml.Unmarshaler] to parse [DefParameter].
func (p *DefParameter) UnmarshalYAML(n *yaml.Node) (err error) {
	type yamlT DefParameter
	var y yamlT
	errStr := []string{sErrEmptyActionParamName, sErrInvalidActionParamName, sErrDupActionParamName}

	if err = n.Decode(&y); err != nil {
		return err
	}

	*p = DefParameter(y)
	if p.Name == "" {
		return yamlTypeErrorLine(errStr[0], n.Line, n.Column)
	}
	if !rgxVarName.MatchString(p.Name) {
		l, c := yamlNodeLineCol(n, "name")
		return yamlTypeErrorLine(fmt.Sprintf(errStr[1], p.Name), l, c)
	}
	dups := yamlTree.dupsByNode(n)
	if !dups.isUnique(p.Name) {
		l, c := yamlNodeLineCol(n, "name")
		return yamlTypeErrorLine(fmt.Sprintf(errStr[2], p.Name), l, c)
	}
	if err = n.Decode(&p.raw); err != nil {
		return err
	}
	if p.Type == "" {
		p.Type = jsonschema.String
	}
	if p.Title == "" {
		p.Title = p.Name
	}
	// Cast enum any to expected type, make sure enum is correctly filled.
	for i := 0; i < len(p.Enum); i++ {
		v, err := jsonschema.EnsureType(p.Type, p.Enum[i])
		if err != nil {
			enumNode := yamlFindNodeByKey(n, "enum")
			return yamlTypeErrorLine(err.Error(), enumNode.Line, enumNode.Column)
		}
		p.Enum[i] = v
	}
	p.raw["type"] = p.Type
	if p.Type == jsonschema.Array {
		// Force default array's "items" type declaration if not specified.
		if p.Items == nil {
			p.Items = &DefArrayItems{Type: jsonschema.String}
			p.raw["items"] = map[string]any{
				"type": jsonschema.String,
			}
		}
	}

	// Set default values.
	_, okDef := p.raw["default"]
	if okDef {
		// Ensure default value respects the type.
		dval, errDef := jsonschema.EnsureType(p.Type, p.Default)
		if errDef != nil {
			l, c := yamlNodeLineCol(n, "default")
			return yamlTypeErrorLine(errDef.Error(), l, c)
		}
		p.Default = dval
		p.raw["default"] = p.Default
	}

	// Not JSONSchema properties.
	delete(p.raw, "name")
	delete(p.raw, "shorthand")
	delete(p.raw, "required")
	delete(p.raw, "process")

	return nil
}

func unmarshalParamListYaml(nl *yaml.Node) ([]*DefParameter, error) {
	if nl.Kind != yaml.SequenceNode {
		return nil, yamlTypeErrorLine(sErrFieldMustBeArr, nl.Line, nl.Column)
	}
	l := make([]*DefParameter, 0, len(nl.Content))
	var errs *yaml.TypeError
	for _, node := range nl.Content {
		if node.Kind != yaml.MappingNode {
			errs = yamlMergeErrors(errs, yamlTypeErrorLine(sErrArrElMustBeObj, node.Line, node.Column))
			continue
		}
		var v *DefParameter
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

// DefArrayItems stores array type related information.
type DefArrayItems struct {
	Type jsonschema.Type `yaml:"type"`
}

// DefValueProcessor stores information about processor and options that should be applied to processor.
type DefValueProcessor struct {
	ID      string     `yaml:"processor"`
	optsRaw *yaml.Node // optsRaw is saved for later processing of options.
}

// UnmarshalYAML implements [yaml.Unmarshaler] to parse [DefValueProcessor].
func (p *DefValueProcessor) UnmarshalYAML(n *yaml.Node) (err error) {
	type yamlT DefValueProcessor
	var y yamlT
	if err = n.Decode(&y); err != nil {
		return err
	}
	*p = DefValueProcessor(y)
	if p.ID == "" {
		return yamlTypeErrorLine(sErrEmptyProcessorID, n.Line, n.Column)
	}
	p.optsRaw = yamlFindNodeByKey(n, "options")
	return nil
}

// InitProcessors creates [ValueProcessor] handlers according to the definition.
func (p *DefParameter) InitProcessors(list map[string]ValueProcessor) (err error) {
	p.processors, err = initValueProcessors(list, p)
	return err
}
