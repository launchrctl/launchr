// Package builtinprocessors is a plugin of launchr to provide native action processors.
package builtinprocessors

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/knadh/koanf"
	yamlparser "github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/rawbytes"

	"github.com/launchrctl/launchr/internal/launchr"
	"github.com/launchrctl/launchr/pkg/action"
	"github.com/launchrctl/launchr/pkg/jsonschema"
)

const (
	procGetConfigValue = "config.GetValue"
)

func init() {
	launchr.RegisterPlugin(Plugin{})
}

// Plugin is [launchr.Plugin] to provide action processors.
type Plugin struct{}

// PluginInfo implements [launchr.Plugin] interface.
func (p Plugin) PluginInfo() launchr.PluginInfo {
	return launchr.PluginInfo{}
}

// OnAppInit implements [launchr.OnAppInitPlugin] interface.
func (p Plugin) OnAppInit(app launchr.App) error {
	// Get services.
	var cfg launchr.Config
	var tp *action.TemplateProcessors
	app.Services().Get(&cfg)
	app.Services().Get(&tp)

	addValueProcessors(tp, cfg)

	// @todo show somehow available processors to developer or document it.

	return nil
}

// ConfigGetProcessorOptions is an options struct for `config.GetValue`.
type ConfigGetProcessorOptions = *action.GenericValueProcessorOptions[struct {
	Path string `yaml:"path" validate:"not-empty"`
}]

// addValueProcessors submits new [action.ValueProcessor] to [action.Manager].
func addValueProcessors(tp *action.TemplateProcessors, cfg launchr.Config) {
	procCfg := action.GenericValueProcessor[ConfigGetProcessorOptions]{
		Fn: func(v any, opts ConfigGetProcessorOptions, ctx action.ValueProcessorContext) (any, error) {
			return processorConfigGetByKey(v, opts, ctx, cfg)
		},
	}
	tp.AddValueProcessor(procGetConfigValue, procCfg)
	tplCfg := &configTemplateFunc{cfg: cfg}
	tp.AddTemplateFunc("config", tplCfg.Get)
	tp.AddTemplateFunc("yq", func(ctx action.TemplateFuncContext) any {
		tplYq := &yamlQueryTemplateFunc{action: ctx.Action()}
		return tplYq.Get
	})
}

func processorConfigGetByKey(v any, opts ConfigGetProcessorOptions, ctx action.ValueProcessorContext, cfg launchr.Config) (any, error) {
	// If value is provided by user, do not override.
	if ctx.IsChanged {
		return v, nil
	}

	// Get value from the config.
	var res any
	err := cfg.Get(opts.Fields.Path, &res)
	if err != nil {
		return v, err
	}

	return jsonschema.EnsureType(ctx.DefParam.Type, res)
}

// tplKeyNotFound holds a key path element that was not found.
// It will print a message in a template when a key is missing.
type tplKeyNotFound string

// IsEmpty implements a special interface to support "default" template function
// Example: {{ Config "foo.bar" | default "buz" }}
func (s tplKeyNotFound) IsEmpty() bool { return true }

// String implements [fmt.Stringer] to output a missing key to a template.
func (s tplKeyNotFound) String() string { return "<key not found \"" + string(s) + "\">" }

// configTemplateFunc is a set of template functions to interact with [launchr.Config] in [action.TemplateProcessors].
type configTemplateFunc struct {
	cfg launchr.Config
}

// Get returns a config value by a path.
//
// Usage:
//
//	{{ config "foo.bar" }} - retrieves value of any type
//	{{ index (config "foo.array-elem") 1 }} - retrieves specific array element
//	{{ config "foo.null-elem" | default "foo" }} - uses default if value is nil
//	{{ config "foo.missing-elem" | default "bar" }} - uses default if key doesn't exist
func (t *configTemplateFunc) Get(path string) (any, error) {
	var res any
	if !t.cfg.Exists(path) {
		return tplKeyNotFound(path), nil
	}
	err := t.cfg.Get(path, &res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// yamlQueryTemplateFunc is a set of template funciton to parse and query yaml files like `yq`.
type yamlQueryTemplateFunc struct {
	action *action.Action
}

// Get returns a yaml file value by a key path.
//
// Usage:
//
//	{{ yq "foo.bar" }} - retrieves value of any type
//	{{ index (yq "foo.array-elem") 1 }} - retrieves specific array element
//	{{ yq "foo.null-elem" | default "foo" }} - uses default if value is nil
//	{{ yq "foo.missing-elem" | default "bar" }} - uses default if key doesn't exist
func (t *yamlQueryTemplateFunc) Get(filename, key string) (any, error) {
	k := koanf.New(".")
	absPath := filepath.ToSlash(filename)
	if !filepath.IsAbs(absPath) {
		absPath = filepath.Join(t.action.WorkDir(), absPath)
	}

	content, err := os.ReadFile(absPath) //nolint:gosec // G301 File inclusion is expected.
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("can't find yaml file %q", filename)
		}
		return nil, fmt.Errorf("can't read yaml file %q: %w", filename, err)
	}

	err = k.Load(rawbytes.Provider(content), yamlparser.Parser())
	if err != nil {
		return nil, err
	}

	if !k.Exists(key) {
		return tplKeyNotFound(filename + ":" + key), nil
	}

	val := k.Get(key)
	return val, nil
}
