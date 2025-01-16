package action

import (
	"fmt"

	"github.com/launchrctl/launchr/pkg/jsonschema"
)

const (
	jsonschemaPropArgs = "arguments"
	jsonschemaPropOpts = "options"
)

// validateJSONSchema validates arguments and options according to
// a specified json schema in action definition.
func validateJSONSchema(a *Action, input *Input) error {
	return jsonschema.Validate(
		a.JSONSchema(),
		map[string]any{
			jsonschemaPropArgs: input.ArgsNamed(),
			jsonschemaPropOpts: input.OptsAll(),
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
	return copyMap(p.raw)
}
