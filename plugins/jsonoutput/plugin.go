// Package jsonoutput is a plugin to enable structured output (JSON/YAML) for actions.
package jsonoutput

import (
	"math"

	"github.com/launchrctl/launchr/internal/launchr"
	"github.com/launchrctl/launchr/pkg/action"
	"github.com/launchrctl/launchr/pkg/jsonschema"
)

func init() {
	launchr.RegisterPlugin(&Plugin{})
}

// Plugin is [launchr.Plugin] to enable structured output for actions.
type Plugin struct{}

// PluginInfo implements [launchr.Plugin] interface.
func (p Plugin) PluginInfo() launchr.PluginInfo {
	return launchr.PluginInfo{
		Weight: math.MinInt + 1, // Run early, after verbosity plugin.
	}
}

// OnAppInit implements [launchr.OnAppInitPlugin] interface.
func (p Plugin) OnAppInit(app launchr.App) error {
	outputFormat := ""

	// Assert we are able to access internal functionality.
	appInternal, ok := app.(launchr.AppInternal)
	if !ok {
		return nil
	}

	// Define output format flags.
	cmd := appInternal.RootCmd()
	pflags := cmd.PersistentFlags()
	// Make sure not to fail on unknown flags because we are parsing early.
	unkFlagsBkp := pflags.ParseErrorsAllowlist.UnknownFlags
	pflags.ParseErrorsAllowlist.UnknownFlags = true
	pflags.StringVarP(&outputFormat, "output", "o", "", "output action result in specified format: json or yaml (requires action to define result schema)")

	// Parse available flags.
	err := pflags.Parse(appInternal.CmdEarlyParsed().Args)
	if launchr.IsCommandErrHelp(err) {
		return nil
	}
	if err != nil {
		// It shouldn't happen here.
		panic(err)
	}
	pflags.ParseErrorsAllowlist.UnknownFlags = unkFlagsBkp

	var am action.Manager
	app.Services().Get(&am)

	// Retrieve and expand application persistent flags with output format options.
	persistentFlags := am.GetPersistentFlags()
	persistentFlags.AddDefinitions(getOutputPersistentFlags())

	// Store initial value of the flag.
	persistentFlags.Set("output", outputFormat)

	return nil
}

func getOutputPersistentFlags() action.ParametersList {
	return action.ParametersList{
		&action.DefParameter{
			Name:        "output",
			Title:       "Output Format",
			Description: "Output action result in specified format: json or yaml (requires action to define result schema)",
			Type:        jsonschema.String,
			Default:     "",
		},
	}
}
