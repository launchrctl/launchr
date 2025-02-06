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
	app.GetService(&cfg)
	app.GetService(&am)

	addValueProcessors(am, cfg)

	// @todo show somehow available processors to developer or document it.

	return nil
}

// ConfigGetProcessorOptions is an options struct for `config.GetValue`.
type ConfigGetProcessorOptions = *action.GenericValueProcessorOptions[struct {
	Path string `yaml:"path" validate:"not-empty"`
}]

// addValueProcessors submits new [action.ValueProcessor] to [action.Manager].
func addValueProcessors(m action.Manager, cfg launchr.Config) {
	procCfg := action.GenericValueProcessor[ConfigGetProcessorOptions]{
		Fn: func(v any, opts ConfigGetProcessorOptions, ctx action.ValueProcessorContext) (any, error) {
			return processorConfigGetByKey(v, opts, ctx, cfg)
		},
	}
	m.AddValueProcessor(procGetConfigValue, procCfg)
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
