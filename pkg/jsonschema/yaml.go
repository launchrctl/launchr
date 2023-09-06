package jsonschema

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// UnmarshalYAML implements yaml.Unmarshaler to parse Json Schema type.
func (t *Type) UnmarshalYAML(n *yaml.Node) (err error) {
	var s string
	err = n.Decode(&s)
	if err != nil {
		return err
	}
	st := TypeFromString(s)
	if st == Unsupported {
		return &yaml.TypeError{
			Errors: []string{
				fmt.Sprintf("json schema type %q is unsupported, line %d, col %d", s, n.Line, n.Column),
			},
		}
	}
	*t = st
	return nil
}
