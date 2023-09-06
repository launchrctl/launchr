package action

import "github.com/launchrctl/launchr/pkg/jsonschema"

// JSONSchema returns json schema of an action.
func (a *Action) JSONSchema() jsonschema.Schema {
	s := a.ActionDef().JSONSchema()
	s.ID = a.ID // @todo provide better id.
	s.Schema = "https://json-schema.org/draft/2020-12/schema"
	s.Title = a.ID // @todo provide better title.
	return s
}

// JSONSchema returns jsonschema for the arguments and options of the action.
func (a *DefAction) JSONSchema() jsonschema.Schema {
	// @todo maybe it should return only properties and not schema.
	args, argsReq := a.Arguments.JSONSchema()
	opts, optsReq := a.Options.JSONSchema()

	return jsonschema.Schema{
		Type:     jsonschema.Object,
		Required: []string{"arguments"},
		Properties: map[string]interface{}{
			"arguments": map[string]interface{}{
				"type":       "object",
				"properties": args,
				"required":   argsReq,
			},
			"options": map[string]interface{}{
				"type":       "object",
				"properties": opts,
				"required":   optsReq,
			},
		},
	}
}

// JSONSchema collects all arguments json schema definition and also returns fields that are required.
func (l *ArgumentsList) JSONSchema() (map[string]interface{}, []string) {
	s := *l
	args := make(map[string]interface{}, len(s))
	req := make([]string, 0, len(s))
	for i := 0; i < len(s); i++ {
		args[s[i].Name] = s[i].JSONSchema()
		req = append(req, s[i].Name)
	}
	return args, req
}

// JSONSchema returns argument json schema definition.
func (a *Argument) JSONSchema() map[string]interface{} {
	return copyMap(a.RawMap)
}

// JSONSchema collects all options json schema definition and also returns fields that are required.
func (l *OptionsList) JSONSchema() (map[string]interface{}, []string) {
	s := *l
	opts := make(map[string]interface{}, len(s))
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
func (o *Option) JSONSchema() map[string]interface{} {
	return copyMap(o.RawMap)
}
