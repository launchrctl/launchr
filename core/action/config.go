package action

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/launchrctl/launchr/core/cli"
	"github.com/launchrctl/launchr/core/jsonschema"
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

// Config is a representation of an action yaml
type Config struct {
	Version string  `yaml:"version"`
	Action  *Action `yaml:"action"`
}

// StrSlice is an array of strings for command execution.
type StrSlice []string

// EnvSlice is an array of env vars or key-value.
type EnvSlice []string

// Action is a representation of an action structure of an action yaml
type Action struct {
	Title       string               `yaml:"title"`
	Description string               `yaml:"description"`
	Arguments   ArgumentsList        `yaml:"arguments"`
	Options     OptionsList          `yaml:"options"`
	Command     StrSlice             `yaml:"command"`
	Image       string               `yaml:"image"`
	Build       *cli.BuildDefinition `yaml:"build"`
	ExtraHosts  []string             `yaml:"extra_hosts"`
	Env         EnvSlice             `yaml:"env"`
}

// BuildDefinition provides resolved image build info
func (a *Action) BuildDefinition(wd string) *cli.BuildDefinition {
	return cli.PrepareImageBuildDefinition(wd, a.Build, a.Image)
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

func validateV1(cfg *Config) error {
	// @todo validate somehow
	if cfg.Action.Image == "" {
		return errEmptyActionImg
	}
	if len(cfg.Action.Command) == 0 {
		return errEmptyActionCmd
	}
	nameRgx := regexp.MustCompile("^[a-zA-Z](?:_?[a-zA-Z0-9]+)*$")
	dups := make(dupSet, (len(cfg.Action.Arguments)+len(cfg.Action.Options))*2)
	// Validate all arguments have name.
	for _, a := range cfg.Action.Arguments {
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
	for _, o := range cfg.Action.Options {
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

func (c *Config) initDefaults() {
	// Set default version to 1
	if c.Version == "" {
		c.Version = "1"
	}
	for _, a := range c.Action.Arguments {
		if a.Type == "" {
			a.Type = jsonschema.String
		}
	}

	for _, o := range c.Action.Options {
		if o.Type == "" {
			o.Type = jsonschema.String
		}
		o.Default = getDefaultByType(o)
	}
}

func getDefaultByType(o *Option) interface{} {
	switch o.Type {
	case jsonschema.String:
		return getDefaultForInterface(o.Default, "")
	case jsonschema.Integer:
		return getDefaultForInterface(o.Default, 0)
	case jsonschema.Number:
		return getDefaultForInterface(o.Default, .0)
	case jsonschema.Boolean:
		return getDefaultForInterface(o.Default, false)
	case jsonschema.Array:
		return getDefaultForInterface(o.Default, []string{})
	default:
		panic(fmt.Sprintf("json schema type %s is not implemented", o.Type))
	}
}

func getDefaultForInterface[T any](val interface{}, d T) T {
	if val != nil {
		return val.(T)
	}
	return d
}
