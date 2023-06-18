package jsonschema

import (
	"errors"

	"gopkg.in/yaml.v3"
)

var (
	errUnsupportedType = errors.New("json schema type is unsupported")
)

// UnmarshalYAML implements yaml.Unmarshaler to parse Json Schema type.
func (t *Type) UnmarshalYAML(n *yaml.Node) (err error) {
	var s string
	err = n.Decode(&s)
	if err != nil {
		return err
	}
	st := FromString(s)
	if st == Unsupported {
		return errUnsupportedType
	}
	*t = st
	return nil
}
