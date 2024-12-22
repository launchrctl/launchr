// Package jsonschema has functionality related to json schema support.
package jsonschema

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Type is a json schema type.
type Type string

// Json schema types.
const (
	String      Type = "string"
	Number      Type = "number"
	Integer     Type = "integer"
	Boolean     Type = "boolean"
	Null        Type = "null"
	Object      Type = "object"
	Array       Type = "array"
	Unsupported Type = "UNSUPPORTED"
)

// TypeFromString creates a [Type] with enum validation.
func TypeFromString(t string) Type {
	if t == "" {
		return String
	}
	switch Type(t) {
	case String, Number, Integer, Boolean, Array, Object, Null:
		return Type(t)
	default:
		return Unsupported
	}
}

// TypeDefault returns a default value for a json schema type t if v is not defined.
// Error is returned on type mismatch.
func TypeDefault(t Type, v any) (any, error) {
	switch t {
	case String:
		return defaultVal(v, "")
	case Integer:
		return defaultVal(v, 0)
	case Number:
		return defaultVal(v, .0)
	case Boolean:
		return defaultVal(v, false)
	case Array:
		return defaultVal(v, []any{})
	case Object:
		return defaultVal(v, map[string]any{})
	case Null:
		return nil, nil
	default:
		return nil, fmt.Errorf("default value for json schema type %q is not implemented", t)
	}
}

// MustTypeDefault returns a default value to a type.
func MustTypeDefault(t Type, v any) any {
	res, err := TypeDefault(t, v)
	if err != nil {
		panic(err)
	}
	return res
}

func defaultVal[T any](val any, d T) (T, error) {
	// User default value is not defined, use type default.
	if val == nil {
		return d, nil
	}

	// User value type is of expected type (same as default type).
	switch v := val.(type) {
	case T:
		return v, nil
	}

	return d, fmt.Errorf("value type and expected type mismatch")
}

// ConvertStringToType converts a string value to jsonschema type.
func ConvertStringToType(s string, t Type) (any, error) {
	switch t {
	case String:
		return s, nil
	case Integer:
		return strconv.Atoi(s)
	case Number:
		return strconv.ParseFloat(s, 64)
	case Boolean:
		return strconv.ParseBool(s)
	case Null:
		return nil, nil
	default:
		return nil, fmt.Errorf("cast to json schema type %q is not implemented", t)
	}
}

// Schema is a json schema definition.
// It doesn't implement all and may not comply fully.
// See https://json-schema.org/specification.html
type Schema struct {
	ID          string   `json:"$id,omitempty"`
	Schema      string   `json:"$schema,omitempty"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Type        Type     `json:"type"`
	Required    []string `json:"required"`
	// @todo make a recursive type of properties.
	Properties map[string]any `json:"properties"`
}

// Validate checks if input complies with given schema.
func Validate(s Schema, input map[string]any) error {
	// @todo cache jsonschema and resources.
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}

	schema, err := jsonschema.UnmarshalJSON(bytes.NewBuffer(b))
	if err != nil {
		return err
	}
	c := jsonschema.NewCompiler()
	if err = c.AddResource(s.ID, schema); err != nil {
		return err
	}
	c.AssertFormat()
	sch, err := c.Compile(s.ID)
	if err != nil {
		return err
	}

	err = sch.Validate(input)
	if err == nil {
		return nil
	}
	// Return our error type.
	if errv, okType := err.(*jsonschema.ValidationError); okType {
		return newSchemaValidationErrors(errv)
	}
	return err
}
