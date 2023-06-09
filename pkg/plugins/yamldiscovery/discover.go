// Package yamldiscovery implements a launchr plugin to
// discover actions defined in yaml.
package yamldiscovery

import (
	"io/fs"
	"os"

	"github.com/spf13/cobra"

	"github.com/launchrctl/launchr"
	"github.com/launchrctl/launchr/pkg/action"
	"github.com/launchrctl/launchr/pkg/cli"
)

// ID is a plugin id.
const ID = "actions.yamldiscovery"

func init() {
	launchr.RegisterPlugin(&Plugin{})
}

// Plugin is a plugin to discover actions defined in yaml.
type Plugin struct {
	app *launchr.App
}

// PluginInfo implements launchr.Plugin interface.
func (p *Plugin) PluginInfo() launchr.PluginInfo {
	return launchr.PluginInfo{
		ID: ID,
	}
}

// InitApp implements launchr.Plugin interface to provide discovered actions.
func (p *Plugin) InitApp(app *launchr.App) error {
	p.app = app
	dp := p.app.GetWD()
	appFs := os.DirFS(dp)
	cmds, err := discoverActions(appFs)
	if err != nil {
		return err
	}
	for _, cmdDef := range cmds {
		app.AddActionCommand(cmdDef)
	}
	return nil
}

// CobraAddCommands implements launchr.CobraPlugin interface to provide discovered actions.
func (p *Plugin) CobraAddCommands(rootCmd *cobra.Command) error {
	// CLI command to discover actions in file structure and provide
	var discoverCmd = &cobra.Command{
		Use:   "discover",
		Short: "Discovers available actions in filesystem",
		RunE: func(cmd *cobra.Command, args []string) error {
			dp := p.app.GetWD()
			cmds, err := discoverActions(os.DirFS(dp))
			if err != nil {
				return err
			}

			// @todo cache discovery to read fs only once.
			for _, a := range cmds {
				cli.Println("%s", a.CommandName)
			}

			return nil
		},
	}
	// Discover actions.
	rootCmd.AddCommand(discoverCmd)
	return nil
}

func discoverActions(fs fs.FS) ([]*action.Command, error) {
	return action.NewYamlDiscovery(fs).Discover()
}
