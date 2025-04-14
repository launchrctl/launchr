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

// Clone creates a copy of the [PersistentFlags] instance.
func (p *PersistentFlags) Clone() *PersistentFlags {
	result := NewPersistentFlags()
	result.AddDefinitions(p.definitions)
	for name, value := range p.values {
		result.Set(name, value)
	}
	return result
}

// GetAll returns latest state of flags.
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

// Get returns state of named flag.
// Return false if flag doesn't exist.
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

// Unset removes flag value.
// Default value will be returned during [PersistentFlags.GetAll] if flag is not set.
func (p *PersistentFlags) Unset(name string) {
	delete(p.values, name)
}

// GetDefinitions returns [ParametersList] with flags definitions.
func (p *PersistentFlags) GetDefinitions() ParametersList {
	return p.definitions
}

// AddDefinitions adds new flag definition.
func (p *PersistentFlags) AddDefinitions(opts ParametersList) {
	itemMap := make(map[string]int)

	for index, item := range p.definitions {
		itemMap[item.Name] = index
	}

	for _, item := range opts {
		if item.Name == "" {
			panic("persistent flag name cannot be empty")
		}

		if _, exists := itemMap[item.Name]; exists {
			panic(fmt.Sprintf("duplicate persistent flag has been detected %s", item.Name))
		}

		p.definitions = append(p.definitions, item)
	}

	for _, d := range p.definitions {
		p.defaults[d.Name] = d.Default
	}
}

// ValidateFlags validates input flags.
func (p *PersistentFlags) ValidateFlags(flags InputParams) error {
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

	return jsonschema.Validate(s, map[string]any{jsonschemaPropPersistent: flags})
}
