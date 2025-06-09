package action

import (
	"fmt"
	"maps"

	"github.com/launchrctl/launchr/pkg/jsonschema"
)

const (
	jsonschemaPropArgs       = "arguments"
	jsonschemaPropOpts       = "options"
	jsonschemaPropRuntime    = "runtime"
	jsonschemaPropPersistent = "persistent"
)

// validateJSONSchema validates arguments and options according to
// a specified json schema in action definition.
func validateJSONSchema(a *Action, input *Input) error {
	return jsonschema.Validate(
		a.JSONSchema(),
		map[string]any{
			jsonschemaPropArgs: input.Args(),
			jsonschemaPropOpts: input.Opts(),
		},
	)
}

// JSONSchema returns json schema of an action.
func (a *Action) JSONSchema() jsonschema.Schema {
	def := a.ActionDef()
	s := def.JSONSchema()
	// Set ID to a filepath. It's not exactly JSON Schema, but some canonical path.
	// It's better to override the value, if the ID is needed by a validator.
	// In launchr, the id is overridden on loader, in web plugin with a server url.
	s.ID = a.Filepath()
	// For plugin defined actions, filepath may be empty.
	if s.ID == "" {
		s.ID = a.ID
	}
	s.Schema = "https://json-schema.org/draft/2020-12/schema#"
	s.Title = fmt.Sprintf("%s (%s)", def.Title, a.ID) // @todo provide better title.
	s.Description = def.Description
	return s
}

// JSONSchema returns [jsonschema.Schema] for the arguments and options of the action.
func (a *DefAction) JSONSchema() jsonschema.Schema {
	args, argsReq := a.Arguments.JSONSchema()
	opts, optsReq := a.Options.JSONSchema()

	return jsonschema.Schema{
		Type:     jsonschema.Object,
		Required: []string{jsonschemaPropArgs, jsonschemaPropOpts},
		Properties: map[string]any{
			jsonschemaPropArgs: map[string]any{
				"type":                 "object",
				"title":                "Arguments",
				"properties":           args,
				"required":             argsReq,
				"additionalProperties": false,
			},
			jsonschemaPropOpts: map[string]any{
				"type":                 "object",
				"title":                "Options",
				"properties":           opts,
				"required":             optsReq,
				"additionalProperties": false,
			},
		},
	}
}

// JSONSchema collects all arguments json schema definition and also returns fields that are required.
func (l *ParametersList) JSONSchema() (map[string]any, []string) {
	s := *l
	params := make(map[string]any, len(s))
	req := make([]string, 0, len(s))
	for i := 0; i < len(s); i++ {
		params[s[i].Name] = s[i].JSONSchema()
		if s[i].Required {
			req = append(req, s[i].Name)
		}
	}
	return params, req
}

// JSONSchema returns json schema definition of an option.
func (p *DefParameter) JSONSchema() map[string]any {
	if len(p.raw) != 0 {
		return maps.Clone(p.raw)
	}

	// We copy to raw because the DefParameter can be created directly in runtime/persistent flags.
	// It is different from when we parse a yaml file, that's why the raw may be empty here and needs to be recreated.
	// TODO: Refactor how DefParameter is created in Persistent, Runtime and yaml, unify 2 cases. Maybe rethink how we create a DefParameter.
	raw := make(map[string]any)
	raw["title"] = p.Title
	raw["type"] = p.Type
	raw["default"] = p.Default

	if len(p.Enum) > 0 {
		raw["enum"] = p.Enum
	}
	if p.Description != "" {
		raw["description"] = p.Description
	}

	return raw
}
