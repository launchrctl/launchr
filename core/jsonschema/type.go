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

// FromString creates a Type with enum validation.
func FromString(t string) Type {
	if t == "" {
		return String
	}
	switch Type(t) {
	case String, Number, Integer, Boolean, Array:
		return Type(t)
	default:
		return Unsupported
	}
}
