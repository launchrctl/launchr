package launchr

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sync"

	"github.com/knadh/koanf"
	yamlparser "github.com/knadh/koanf/parsers/yaml"
	fsprovider "github.com/knadh/koanf/providers/fs"
)

var configRegex = regexp.MustCompile(`^config\.(yaml|yml)$`)

// Common errors.
var (
	ErrNoConfigFile = errors.New("config file is not found") // ErrNoConfigFile when config file doesn't exist in FS.
)

// Config is a launchr global config service.
type Config = *config

type cachedProps = map[string]reflect.Value
type config struct {
	mx       sync.Mutex   // mx is a mutex to read/cache values.
	root     fs.FS        // root is a base dir filesystem.
	fname    fs.DirEntry  // fname is a file storing the config.
	rootPath string       // rootPath is a base dir path.
	cached   cachedProps  // cached is a map of cached properties read from a file.
	koanf    *koanf.Koanf // koanf is the driver to read the yaml config.
}

func findConfigFile(root fs.FS) fs.DirEntry {
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

// ConfigFromFS parses launchr app config directory and its content.
func ConfigFromFS(root fs.FS) Config {
	return &config{
		root:     root,
		rootPath: FsRealpath(root),
		cached:   make(cachedProps),
		fname:    findConfigFile(root),
	}
}

func (cfg *config) ServiceInfo() ServiceInfo {
	return ServiceInfo{}
}

func (cfg *config) ServiceCreate(_ *ServiceManager) Service {
	cfgDir := "." + name
	return ConfigFromFS(os.DirFS(cfgDir))
}

// DirPath returns an absolute path to config directory.
func (cfg *config) DirPath() string {
	return cfg.rootPath
}

func (cfg *config) exists(path string) bool {
	return cfg.koanf != nil && cfg.koanf.Exists(path)
}

// Exists checks if key exists in config. Key level delimiter is dot.
// For example - `path.to.something`.
func (cfg *config) Exists(path string) bool {
	var v any
	err := cfg.Get(path, &v)
	if err != nil {
		return false
	}
	return cfg.exists(path)
}

// Get returns a value by key to a parameter v. Parameter v must be a pointer to a value.
// Error may be returned on decode.
func (cfg *config) Get(key string, v any) error {
	cfg.mx.Lock()
	defer cfg.mx.Unlock()
	var err error
	cached, ok := cfg.cached[key]
	if ok {
		err, ok = cached.Interface().(error)
		if ok {
			return err
		}
		reflect.ValueOf(v).Elem().Set(cached)
		return nil
	}

	if cfg.fname != nil && cfg.koanf == nil {
		if err = cfg.parse(); err != nil {
			return err
		}
	}

	ok = cfg.exists(key)
	if !ok {
		// Return default value.
		return nil
	}
	defer func() {
		// Save error result to prevent parsing twice.
		if err != nil {
			cfg.cached[key] = reflect.ValueOf(err)
		}
	}()
	vcopy := reflect.New(reflect.TypeOf(v).Elem()).Elem()

	err = cfg.koanf.UnmarshalWithConf(key, v, koanf.UnmarshalConf{Tag: "yaml"})
	if err != nil {
		// Set value to empty struct not to leak partial parsing to ensure consistent results.
		reflect.ValueOf(v).Elem().Set(vcopy)
		return err
	}
	cfg.cached[key] = reflect.ValueOf(v).Elem()
	return nil
}

func (cfg *config) parse() error {
	if cfg.fname == nil {
		return ErrNoConfigFile
	}

	cfg.koanf = koanf.New(".")
	err := cfg.koanf.Load(fsprovider.Provider(cfg.root, cfg.fname.Name()), yamlparser.Parser())
	if err != nil {
		return err
	}

	return nil
}

// Path provides an absolute path to launchr config directory.
func (cfg *config) Path(parts ...string) string {
	parts = append([]string{cfg.rootPath}, parts...)
	return filepath.Clean(filepath.Join(parts...))
}
