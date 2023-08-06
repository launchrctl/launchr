package action

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"

	"gopkg.in/yaml.v3"

	"github.com/launchrctl/launchr/pkg/jsonschema"
	"github.com/launchrctl/launchr/pkg/types"
)

var (
	errEmptyActionImg       = errors.New("image field cannot be empty")
	errEmptyActionCmd       = errors.New("command field cannot be empty")
	errEmptyActionArgName   = errors.New("action argument name is required")
	errEmptyActionOptName   = errors.New("action option name is required")
	errInvalidActionArgName = errors.New("argument name is not valid")
	errInvalidActionOptName = errors.New("option name is not valid")
	errDuplicateActionName  = errors.New("argument or option is already defined")
)

var nameRgx = regexp.MustCompile("^[a-zA-Z](?:_?[a-zA-Z0-9]+)*$")

type errUnsupportedActionVersion struct {
	version string
}

// Error implements error interface.
func (err errUnsupportedActionVersion) Error() string {
	return fmt.Sprintf("unsupported version \"%s\" of action file", err.version)
}

// Is implements errors.Is interface.
func (err errUnsupportedActionVersion) Is(cmp error) bool {
	errCmp, ok := cmp.(errUnsupportedActionVersion)
	return ok && errCmp == err
}

// Action is a representation of an action yaml file
type Action struct {
	Version string        `yaml:"version"`
	Action  *ActionConfig `yaml:"action"`
}

// Content implements Loader interface.
func (a *Action) Content() ([]byte, error) {
	w := &bytes.Buffer{}
	err := yaml.NewEncoder(w).Encode(a)
	return w.Bytes(), err
}

// Load implements Loader interface.
func (a *Action) Load() (*Action, error) {
	return a, nil
}

// LoadRaw implements Loader interface.
func (a *Action) LoadRaw() (*Action, error) {
	return a.Load()
}

// StrSlice is an array of strings for command execution.
type StrSlice []string

// EnvSlice is an array of env vars or key-value.
type EnvSlice []string

// ActionConfig holds action configuration
type ActionConfig struct { //nolint:revive
	Title       string                 `yaml:"title"`
	Description string                 `yaml:"description"`
	Arguments   ArgumentsList          `yaml:"arguments"`
	Options     OptionsList            `yaml:"options"`
	Command     StrSlice               `yaml:"command"`
	Image       string                 `yaml:"image"`
	Build       *types.BuildDefinition `yaml:"build"`
	ExtraHosts  []string               `yaml:"extra_hosts"`
	Env         EnvSlice               `yaml:"env"`
}

// BuildDefinition provides resolved image build info
func (a *ActionConfig) BuildDefinition(wd string) *types.BuildDefinition {
	return a.Build.BuildImageInfo(a.Image, wd)
}

// ArgumentsList is used for custom yaml parsing of arguments list.
type ArgumentsList []*Argument

// Argument stores command arguments declaration.
type Argument struct {
	Name        string          `yaml:"name"`
	Title       string          `yaml:"title"`
	Description string          `yaml:"description"`
	Type        jsonschema.Type `yaml:"type"`
	RawMap      map[string]interface{}
}

// OptionsList is used for custom yaml parsing of options list.
type OptionsList []*Option

// Option stores command options declaration.
type Option struct {
	Name        string          `yaml:"name"`
	Title       string          `yaml:"title"`
	Description string          `yaml:"description"`
	Type        jsonschema.Type `yaml:"type"`
	Default     interface{}     `yaml:"default"`
	Required    bool            `yaml:"required"`
	RawMap      map[string]interface{}
}

type dupSet map[string]struct{}

func (d dupSet) exists(k string) bool {
	_, ok := d[k]
	return ok
}

func (d dupSet) add(k string) {
	d[k] = struct{}{}
}

func validateV1(a *Action) error {
	// @todo validate somehow
	// @todo maybe use https://github.com/go-playground/validator
	if a.Action.Image == "" {
		return errEmptyActionImg
	}
	if len(a.Action.Command) == 0 {
		return errEmptyActionCmd
	}
	dups := make(dupSet, (len(a.Action.Arguments)+len(a.Action.Options))*2)
	// Validate all arguments have name.
	for _, a := range a.Action.Arguments {
		if a.Name == "" {
			return errEmptyActionArgName
		}
		if !nameRgx.MatchString(a.Name) {
			return errInvalidActionArgName
		}
		// Make sure the name is unique.
		if dups.exists(a.Name) {
			return errDuplicateActionName
		}
		dups.add(a.Name)
	}
	// Validate all options have name.
	for _, o := range a.Action.Options {
		if o.Name == "" {
			return errEmptyActionOptName
		}
		if !nameRgx.MatchString(o.Name) {
			return errInvalidActionOptName
		}
		// Make sure the name is unique.
		if dups.exists(o.Name) {
			return errDuplicateActionName
		}
		dups.add(o.Name)
	}

	return nil
}

func (a *Action) initDefaults() {
	// Set default version to 1
	if a.Version == "" {
		a.Version = "1"
	}
	for _, a := range a.Action.Arguments {
		if a.Type == "" {
			a.Type = jsonschema.String
		}
	}

	for _, o := range a.Action.Options {
		if o.Type == "" {
			o.Type = jsonschema.String
		}
		o.Default = getDefaultByType(o)
	}
}

func getDefaultByType(o *Option) interface{} {
	switch o.Type {
	case jsonschema.String:
		return defaultVal(o.Default, "")
	case jsonschema.Integer:
		return defaultVal(o.Default, 0)
	case jsonschema.Number:
		return defaultVal(o.Default, .0)
	case jsonschema.Boolean:
		return defaultVal(o.Default, false)
	case jsonschema.Array:
		return defaultVal(o.Default, []string{})
	default:
		// @todo is it ok to panic? The data comes from user input.
		panic(fmt.Sprintf("json schema type %s is not implemented", o.Type))
	}
}

func defaultVal[T any](val interface{}, d T) T {
	if val != nil {
		return val.(T)
	}
	return d
}
