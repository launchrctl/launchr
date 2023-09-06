// Package jsonschema has functionality related to json schema support.
package jsonschema

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

// TypeFromString creates a Type with enum validation.
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

// Schema is a json schema definition.
// It doesn't implement all and may not comply fully.
// See https://json-schema.org/specification.html
type Schema struct {
	ID       string   `json:"$id"`
	Schema   string   `json:"$schema"`
	Title    string   `json:"title"`
	Type     Type     `json:"type"`
	Required []string `json:"required"`
	// @todo make a recursive type of properties.
	Properties map[string]interface{} `json:"properties"`
}
