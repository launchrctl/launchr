// Package config provides global app config object.
package config

import (
	"errors"
	"io"
	"io/fs"
	"path/filepath"
	"reflect"
	"regexp"

	"gopkg.in/yaml.v3"

	"github.com/launchrctl/launchr/internal/launchr"
)

var configRegex = regexp.MustCompile(`^config\.(yaml|yml)$`)

var (
	errNoFile = errors.New("config file is not found")
)

// GlobalConfig is a config interface.
type GlobalConfig interface {
	launchr.Service
	// DirPath returns an absolute path to config directory.
	DirPath() string
	// Path provides an absolute path to global config.
	Path(parts ...string) string
	// EnsurePath creates all directories in the path.
	EnsurePath(parts ...string) error
	// Get returns a value by name to a parameter v. Parameter v must be a pointer to a value.
	// Error may be returned on decode.
	Get(name string, v interface{}) error
}

// GlobalConfigAware provides an interface for structs to support global configuration setting.
type GlobalConfigAware interface {
	// SetGlobalConfig sets a global config to the struct.
	SetGlobalConfig(GlobalConfig)
}

type cachedProps map[string]reflect.Value
type globalConfig struct {
	root     fs.FS
	fname    fs.DirEntry
	rootPath string
	cached   cachedProps
	yaml     map[string]yaml.Node
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

// GlobalConfigFromFS parses global app config.
func GlobalConfigFromFS(root fs.FS) GlobalConfig {
	return &globalConfig{
		root:     root,
		rootPath: launchr.GetFsAbsPath(root),
		cached:   make(cachedProps),
		fname:    findConfig(root),
	}
}

func (cfg *globalConfig) ServiceInfo() launchr.ServiceInfo {
	return launchr.ServiceInfo{
		ID: "global_config",
	}
}

func (cfg *globalConfig) DirPath() string {
	return cfg.rootPath
}

func (cfg *globalConfig) Get(name string, v interface{}) error {
	var err error
	cached, ok := cfg.cached[name]
	if ok {
		err, ok = cached.Interface().(error)
		if ok {
			return err
		}
		reflect.ValueOf(v).Elem().Set(cached)
		return nil
	}

	if cfg.fname != nil && cfg.yaml == nil {
		if err = cfg.parse(); err != nil {
			return err
		}
	}
	y, ok := cfg.yaml[name]
	if !ok {
		// Return default value.
		return nil
	}
	defer func() {
		// Save error result to prevent parsing twice.
		if err != nil {
			cfg.cached[name] = reflect.ValueOf(err)
		}
	}()
	vcopy := reflect.New(reflect.TypeOf(v).Elem()).Elem()
	if err = y.Decode(v); err != nil {
		// Set value to empty struct not to leak partial parsing to ensure consistent results.
		reflect.ValueOf(v).Elem().Set(vcopy)
		return err
	}
	cfg.cached[name] = reflect.ValueOf(v).Elem()
	return nil
}

func (cfg *globalConfig) parse() error {
	if cfg.fname == nil {
		return errNoFile
	}
	r, err := cfg.root.Open(cfg.fname.Name())
	if err != nil {
		return err
	}
	defer r.Close()

	d := yaml.NewDecoder(r)
	err = d.Decode(&cfg.yaml)
	if err != nil && err != io.EOF {
		return err
	}
	return nil
}

func (cfg *globalConfig) Path(parts ...string) string {
	parts = append([]string{cfg.rootPath}, parts...)
	return filepath.Clean(filepath.Join(parts...))
}

func (cfg *globalConfig) EnsurePath(parts ...string) error {
	return launchr.EnsurePath(cfg.Path(parts...))
}
