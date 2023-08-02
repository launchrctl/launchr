package cli

import (
	"io/fs"
	"path"
	"regexp"

	"gopkg.in/yaml.v3"
)

var configRegex = regexp.MustCompile(`^config\.(yaml|yml)$`)

// GlobalConfig is a global config structure.
type GlobalConfig struct {
	root   fs.FS
	Images map[string]*BuildDefinition `yaml:"images"`
}

// BuildDefinition stores image build definition.
type BuildDefinition struct {
	Context   string             `yaml:"context"`
	Buildfile string             `yaml:"buildfile"`
	Args      map[string]*string `yaml:"args"`
	Tags      []string           `yaml:"tags"`
}

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

// GlobalConfigFromDir parses global app config.
func GlobalConfigFromDir(root fs.FS) (*GlobalConfig, error) {
	f := findConfig(root)
	if f == nil {
		return nil, nil
	}
	cfg := GlobalConfig{root: root}
	r, err := root.Open(f.Name())
	if err != nil {
		return nil, err
	}
	decoder := yaml.NewDecoder(r)
	err = decoder.Decode(&cfg)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

func findConfig(root fs.FS) fs.DirEntry {
	dir, err := fs.ReadDir(root, ".")
	if err != nil {
		return nil
	}
	for _, f := range dir {
		if !f.IsDir() && configRegex.MatchString(f.Name()) {
			return f
		}
	}
	return nil
}

// ConfigDir returns config dir path.
func (cfg *GlobalConfig) ConfigDir() string {
	return GetFsAbsPath(cfg.root)
}

// ImageBuildInfo retrieves image build info from global config.
func (cfg *GlobalConfig) ImageBuildInfo(image string) *BuildDefinition {
	if b, ok := cfg.Images[image]; ok {
		return PrepareImageBuildDefinition(cfg.ConfigDir(), b, image)
	}
	for _, b := range cfg.Images {
		for _, t := range b.Tags {
			if t == image {
				return PrepareImageBuildDefinition(cfg.ConfigDir(), b, image)
			}
		}
	}
	return nil
}

// PrepareImageBuildDefinition preprocesses build info to be ready for driver.
func PrepareImageBuildDefinition(cwd string, build *BuildDefinition, imageName string) *BuildDefinition {
	if build == nil {
		return nil
	}
	b := *build
	if !path.IsAbs(build.Context) {
		b.Context = path.Join(cwd, b.Context)
	}
	if imageName != "" {
		b.Tags = append(b.Tags, imageName)
	}
	return &b
}
