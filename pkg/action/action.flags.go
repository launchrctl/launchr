package action

import (
	"fmt"

	"github.com/launchrctl/launchr/pkg/jsonschema"
)

// FlagsGroup holds definitions, current state, and default values of flags.
// @todo think about moving it to new input validation service alongside. See notes in actionManagerMap.ValidateFlags.
type FlagsGroup struct {
	name        string
	definitions ParametersList
	values      map[string]any
	defaults    map[string]any
}

// NewFlagsGroup returns a new instance of [FlagsGroup]
func NewFlagsGroup(name string) *FlagsGroup {
	return &FlagsGroup{
		name:        name,
		definitions: make(ParametersList, 0),
		values:      make(map[string]any),
		defaults:    make(map[string]any),
	}
}

// GetName returns the name of the flags group.
// Deprecated: use Name().
func (p *FlagsGroup) GetName() string {
	return p.Name()
}

// Name returns the name of the flags group.
func (p *FlagsGroup) Name() string {
	return p.name
}

// GetAll returns the latest state of flags.
func (p *FlagsGroup) GetAll() InputParams {
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

func (p *FlagsGroup) exists(name string) bool {
	_, ok := p.defaults[name]
	return ok
}

// Get returns state of a named flag.
// Return false if a flag doesn't exist.
func (p *FlagsGroup) Get(name string) (any, bool) {
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
func (p *FlagsGroup) Set(name string, value any) {
	if !p.exists(name) {
		return
	}

	p.values[name] = value
}

// Unset removes the flag value.
// The default value will be returned during [FlagsGroup.GetAll] if flag is not set.
func (p *FlagsGroup) Unset(name string) {
	delete(p.values, name)
}

// GetDefinitions returns [ParametersList] with flags definitions.
func (p *FlagsGroup) GetDefinitions() ParametersList {
	return p.definitions
}

// AddDefinitions adds new flag definition.
func (p *FlagsGroup) AddDefinitions(opts ParametersList) {
	registered := make(map[string]struct{})

	for _, def := range p.definitions {
		registered[def.Name] = struct{}{}
	}

	for _, opt := range opts {
		if opt.Name == "" {
			panic(fmt.Sprintf("%s flag name cannot be empty", p.name))
		}

		if _, exists := registered[opt.Name]; exists {
			panic(fmt.Sprintf("duplicate %s flag has been detected %s", p.name, opt.Name))
		}

		p.definitions = append(p.definitions, opt)
	}

	for _, d := range p.definitions {
		p.defaults[d.Name] = d.Default
	}
}

// JSONSchema returns JSON schema of a flags group.
func (p *FlagsGroup) JSONSchema() jsonschema.Schema {
	opts, optsReq := p.definitions.JSONSchema()
	return jsonschema.Schema{
		Type:     jsonschema.Object,
		Required: []string{p.name},
		Properties: map[string]any{
			p.name: map[string]any{
				"type":                 "object",
				"title":                p.name,
				"properties":           opts,
				"required":             optsReq,
				"additionalProperties": false,
			},
		},
	}
}

// ValidateFlags validates input flags.
func (p *FlagsGroup) ValidateFlags(params InputParams) error {
	s := p.JSONSchema()
	return jsonschema.Validate(s, map[string]any{p.name: params})
}
