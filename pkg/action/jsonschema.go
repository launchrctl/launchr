package action

import (
	"fmt"

	"github.com/launchrctl/launchr/pkg/jsonschema"
)

// JSONSchema returns json schema of an action.
func (a *Action) JSONSchema() jsonschema.Schema {
	def := a.ActionDef()
	s := def.JSONSchema()
	// Set ID to a filepath. It's not exactly JSON Schema, but some canonical path.
	// It's better to override the value, if the ID is needed by a validator.
	// In launchr, the id is overridden on loader, in web plugin with a server url.
	s.ID = a.Filepath()
	s.Schema = "https://json-schema.org/draft/2020-12/schema#"
	s.Title = fmt.Sprintf("%s (%s)", def.Title, a.ID) // @todo provide better title.
	s.Description = def.Description
	return s
}

// JSONSchema returns [jsonschema.Schema] for the arguments and options of the action.
func (a *DefAction) JSONSchema() jsonschema.Schema {
	// @todo maybe it should return only properties and not schema.
	args, argsReq := a.Arguments.JSONSchema()
	opts, optsReq := a.Options.JSONSchema()

	return jsonschema.Schema{
		Type:     jsonschema.Object,
		Required: []string{"arguments"},
		Properties: map[string]any{
			"arguments": map[string]any{
				"type":       "object",
				"title":      "Arguments",
				"properties": args,
				"required":   argsReq,
			},
			"options": map[string]any{
				"type":       "object",
				"title":      "Options",
				"properties": opts,
				"required":   optsReq,
			},
		},
	}
}

// JSONSchema collects all arguments json schema definition and also returns fields that are required.
func (l *ArgumentsList) JSONSchema() (map[string]any, []string) {
	s := *l
	args := make(map[string]any, len(s))
	req := make([]string, 0, len(s))
	for i := 0; i < len(s); i++ {
		args[s[i].Name] = s[i].JSONSchema()
		req = append(req, s[i].Name)
	}
	return args, req
}

// JSONSchema returns argument json schema definition.
func (a *Argument) JSONSchema() map[string]any {
	m := copyMap(a.RawMap)
	removeRequiredBool(m)
	return m
}

// JSONSchema collects all options json schema definition and also returns fields that are required.
func (l *OptionsList) JSONSchema() (map[string]any, []string) {
	s := *l
	opts := make(map[string]any, len(s))
	req := make([]string, 0, len(s))
	for i := 0; i < len(s); i++ {
		opts[s[i].Name] = s[i].JSONSchema()
		if s[i].Required {
			req = append(req, s[i].Name)
		}
	}
	return opts, req
}

// JSONSchema returns json schema definition of an option.
func (o *Option) JSONSchema() map[string]any {
	m := copyMap(o.RawMap)
	removeRequiredBool(m)
	return m
}

func removeRequiredBool(m map[string]any) {
	// @todo that's not right, but currently the required field in action yaml doesn't comply with json schema.
	delete(m, "required")
}
