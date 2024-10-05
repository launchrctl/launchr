package launchr

import (
	"errors"
	"io/fs"
	"path/filepath"
	"reflect"
	"regexp"
	"sync"

	"github.com/knadh/koanf"
	yamlparser "github.com/knadh/koanf/parsers/yaml"
	fsprovider "github.com/knadh/koanf/providers/fs"
)

var configRegex = regexp.MustCompile(`^config\.(yaml|yml)$`)

var (
	ErrNoConfigFile = errors.New("config file is not found") // ErrNoConfigFile when config file doesn't exist in FS.
)

// Config is a launchr config storage interface.
type Config interface {
	Service
	// DirPath returns an absolute path to config directory.
	DirPath() string
	// Path provides an absolute path to launchr config directory.
	Path(parts ...string) string
	// Exists checks if key exists in config. Key level delimiter is dot.
	// For example - `path.to.something`.
	Exists(key string) bool
	// Get returns a value by key to a parameter v. Parameter v must be a pointer to a value.
	// Error may be returned on decode.
	Get(key string, v any) error
}

// ConfigAware provides an interface for structs to support launchr configuration setting.
type ConfigAware interface {
	// SetLaunchrConfig sets a launchr config to the struct.
	SetLaunchrConfig(Config)
}

type cachedProps = map[string]reflect.Value
type config struct {
	mx       sync.Mutex
	root     fs.FS
	fname    fs.DirEntry
	rootPath string
	cached   cachedProps
	koanf    *koanf.Koanf
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
		rootPath: GetFsAbsPath(root),
		cached:   make(cachedProps),
		fname:    findConfigFile(root),
	}
}

func (cfg *config) ServiceInfo() ServiceInfo {
	return ServiceInfo{}
}

func (cfg *config) DirPath() string {
	return cfg.rootPath
}

func (cfg *config) Exists(path string) bool {
	return cfg.koanf != nil && cfg.koanf.Exists(path)
}

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

	ok = cfg.Exists(key)
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

	err = cfg.koanf.Unmarshal(key, v)
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

func (cfg *config) Path(parts ...string) string {
	parts = append([]string{cfg.rootPath}, parts...)
	return filepath.Clean(filepath.Join(parts...))
}
