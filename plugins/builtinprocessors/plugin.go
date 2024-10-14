// Package builtinprocessors is a plugin of launchr to provide native action processors.
package builtinprocessors

import (
	"fmt"

	"github.com/launchrctl/launchr/internal/launchr"
	"github.com/launchrctl/launchr/pkg/action"
	"github.com/launchrctl/launchr/pkg/jsonschema"
)

const (
	getConfigValue = "launchr.GetConfigValue"
)

func init() {
	launchr.RegisterPlugin(Plugin{})
}

// Plugin is launchr plugin to provide action processors.
type Plugin struct{}

// PluginInfo implements launchr.Plugin interface.
func (p Plugin) PluginInfo() launchr.PluginInfo {
	return launchr.PluginInfo{}
}

// OnAppInit implements launchr.Plugin interface.
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

// AddValueProcessors submits new ValueProcessors to action.Manager.
func addValueProcessors(m action.Manager, cfg launchr.Config) {
	getByKey := func(value any, options map[string]any) (any, error) {
		return getByKeyProcessor(value, options, cfg)
	}

	proc := action.NewFuncProcessor(
		[]jsonschema.Type{jsonschema.String, jsonschema.Integer, jsonschema.Boolean, jsonschema.Number},
		getByKey,
	)
	m.AddValueProcessor(getConfigValue, proc)
}

func getByKeyProcessor(value any, options map[string]any, cfg launchr.Config) (any, error) {
	if value != nil {
		launchr.Term().Warning().Printfln("Skipping processor %q, value is not empty. Value will remain unchanged", getConfigValue)
		launchr.Log().Warn("skipping processor, value is not empty", "processor", getConfigValue)
		return value, nil
	}

	path, ok := options["path"].(string)
	if !ok {
		return value, fmt.Errorf(`option "path" is required for %q processor`, getConfigValue)
	}

	var res any
	err := cfg.Get(path, &res)
	if err != nil {
		return value, err
	}

	switch res.(type) {
	case int, int8, int16, int32, int64, float32, float64, string, bool:
		value = res
	}

	return value, nil
}
