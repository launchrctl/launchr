package action

import (
	"fmt"

	"github.com/launchrctl/launchr/pkg/jsonschema"
)

// PersistentFlags holds definitions, current state, and default values of flags.
type PersistentFlags struct {
	definitions ParametersList
	values      map[string]any
	defaults    map[string]any
}

// NewPersistentFlags returns new instance of [PersistentFlags]
func NewPersistentFlags() *PersistentFlags {
	return &PersistentFlags{
		definitions: make(ParametersList, 0),
		values:      make(map[string]any),
		defaults:    make(map[string]any),
	}
}

// GetAll returns the latest state of flags.
func (p *PersistentFlags) GetAll() InputParams {
	result := make(InputParams)
	for name, value := range p.defaults {
		if _, ok := p.values[name]; !ok {
			result[name] = value
		} else {
			result[name] = p.values[name]
		}
	}

	return result
}

func (p *PersistentFlags) exists(name string) bool {
	_, ok := p.defaults[name]
	return ok
}

// Get returns state of a named flag.
// Return false if a flag doesn't exist.
func (p *PersistentFlags) Get(name string) (any, bool) {
	if !p.exists(name) {
		return nil, false
	}

	var value any
	if v, ok := p.values[name]; ok {
		value = v
	} else {
		value = p.defaults[name]
	}

	return value, true
}

// Set sets new state value for a flag. Does nothing if flag doesn't exist.
func (p *PersistentFlags) Set(name string, value any) {
	if !p.exists(name) {
		return
	}

	p.values[name] = value
}

// Unset removes the flag value.
// The default value will be returned during [PersistentFlags.GetAll] if flag is not set.
func (p *PersistentFlags) Unset(name string) {
	delete(p.values, name)
}

// GetDefinitions returns [ParametersList] with flags definitions.
func (p *PersistentFlags) GetDefinitions() ParametersList {
	return p.definitions
}

// AddDefinitions adds new flag definition.
func (p *PersistentFlags) AddDefinitions(opts ParametersList) {
	registered := make(map[string]struct{})

	for _, def := range p.definitions {
		registered[def.Name] = struct{}{}
	}

	for _, opt := range opts {
		if opt.Name == "" {
			panic("persistent flag name cannot be empty")
		}

		if _, exists := registered[opt.Name]; exists {
			panic(fmt.Sprintf("duplicate persistent flag has been detected %s", opt.Name))
		}

		p.definitions = append(p.definitions, opt)
	}

	for _, d := range p.definitions {
		p.defaults[d.Name] = d.Default
	}
}

// ValidateInput validates input flags.
func (p *PersistentFlags) ValidateInput(_ *Action, input *Input) error {
	// @todo move to separate service with full input validation and maybe combine with runtime input check.
	opts, optsReq := p.definitions.JSONSchema()
	s := jsonschema.Schema{
		Type:     jsonschema.Object,
		Required: []string{jsonschemaPropPersistent},
		Properties: map[string]any{
			jsonschemaPropPersistent: map[string]any{
				"type":                 "object",
				"title":                jsonschemaPropPersistent,
				"properties":           opts,
				"required":             optsReq,
				"additionalProperties": false,
			},
		},
	}

	return jsonschema.Validate(s, map[string]any{jsonschemaPropPersistent: input.PersistentFlags()})
}
