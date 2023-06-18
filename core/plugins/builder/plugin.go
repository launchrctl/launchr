// Package builder implements a plugin to build launchr with plugins.
package builder

import (
	"github.com/spf13/cobra"

	"github.com/launchrctl/launchr"
)

// ID is a plugin id.
const ID = "builder"

func init() {
	launchr.RegisterPlugin(&Plugin{})
}

// Plugin is a plugin to build launchr application.
type Plugin struct {
}

// PluginInfo implements launchr.Plugin interface.
func (p *Plugin) PluginInfo() launchr.PluginInfo {
	return launchr.PluginInfo{
		ID: ID,
	}
}

// InitApp implements launchr.Plugin interface.
func (p *Plugin) InitApp(*launchr.App) error {
	return nil
}

// CobraAddCommands implements launchr.CobraPlugin interface to provide build functionality.
func (p *Plugin) CobraAddCommands(rootCmd *cobra.Command) error {
	// Flag options.
	var (
		name    string
		out     string
		plugins []string
		replace []string
		debug   bool
	)

	var buildCmd = &cobra.Command{
		Use: "build",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Don't show usage help on a runtime error.
			cmd.SilenceUsage = true
			return Execute(name, out, plugins, replace, launchr.GetVersion(), debug)
		},
	}
	buildCmd.Flags().StringVarP(&name, "name", "n", "launchr", `Result application name`)
	buildCmd.Flags().StringVarP(&out, "output", "o", "", `Build output file, by default application name is used"`)
	buildCmd.Flags().StringSliceVarP(&plugins, "plugin", "p", nil, `Include PLUGIN into the build with an optional version`)
	buildCmd.Flags().StringSliceVarP(&replace, "replace", "r", nil, `Replace go dependency, see "go mod edit -replace"`)
	buildCmd.Flags().BoolVarP(&debug, "debug", "d", false, `Include debug flags into the build to support go debugging with "delve". If not specified, debugging info is trimmed`)
	rootCmd.AddCommand(buildCmd)
	return nil
}
