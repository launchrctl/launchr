package cli

import "gopkg.in/yaml.v3"

type yamlBuildOptions BuildDefinition

// UnmarshalYAML implements yaml.Unmarshaler to parse build options from a string or a struct.
func (l *BuildDefinition) UnmarshalYAML(n *yaml.Node) (err error) {
	if n.Kind == yaml.ScalarNode {
		var s string
		err = n.Decode(&s)
		*l = BuildDefinition{Context: s}
		return err
	}
	var s yamlBuildOptions
	err = n.Decode(&s)
	if err != nil {
		return err
	}
	*l = BuildDefinition(s)
	return err
}
