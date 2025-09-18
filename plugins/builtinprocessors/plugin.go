// Package builtinprocessors is a plugin of launchr to provide native action processors.
package builtinprocessors

import (
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
	var am action.Manager
	app.Services().Get(&cfg)
	app.Services().Get(&am)

	addValueProcessors(am, cfg)

	// @todo show somehow available processors to developer or document it.

	return nil
}

// ConfigGetProcessorOptions is an options struct for `config.GetValue`.
type ConfigGetProcessorOptions = *action.GenericValueProcessorOptions[struct {
	Path string `yaml:"path" validate:"not-empty"`
}]

// addValueProcessors submits new [action.ValueProcessor] to [action.Manager].
func addValueProcessors(tp action.TemplateProcessors, cfg launchr.Config) {
	procCfg := action.GenericValueProcessor[ConfigGetProcessorOptions]{
		Fn: func(v any, opts ConfigGetProcessorOptions, ctx action.ValueProcessorContext) (any, error) {
			return processorConfigGetByKey(v, opts, ctx, cfg)
		},
	}
	tp.AddValueProcessor(procGetConfigValue, procCfg)
	tplCfg := &configTemplateFunc{cfg: cfg}
	tp.AddTemplateFunc("config", func() *configTemplateFunc { return tplCfg })
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

// configKeyNotFound holds a config key element that was not found in config.
// It will print a message in a template when a config key is missing.
type configKeyNotFound string

// IsEmpty implements a special interface to support "default" template function
// Example: {{ config.Get "foo.bar" | default "buz" }}
func (s configKeyNotFound) IsEmpty() bool { return true }

// String implements [fmt.Stringer] to output a missing key to a template.
func (s configKeyNotFound) String() string { return "<config key not found \"" + string(s) + "\">" }

// configTemplateFunc is a set of template functions to interact with [launchr.Config] in [action.TemplateProcessors].
type configTemplateFunc struct {
	cfg launchr.Config
}

// Get returns a config value by a path.
//
// Usage:
//
//	{{ config.Get "foo.bar" }} - retrieves value of any type
//	{{ index (config.Get "foo.array-elem") 1 }} - retrieves specific array element
//	{{ config.Get "foo.null-elem" | default "foo" }} - uses default if value is nil
//	{{ config.Get "foo.missing-elem" | default "bar" }} - uses default if key doesn't exist
func (t *configTemplateFunc) Get(path string) (any, error) {
	var res any
	if !t.cfg.Exists(path) {
		return configKeyNotFound(path), nil
	}
	err := t.cfg.Get(path, &res)
	if err != nil {
		return nil, err
	}
	return res, nil
}
