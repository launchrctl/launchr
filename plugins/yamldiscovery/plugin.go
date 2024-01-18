// Package yamldiscovery implements a launchr plugin to
// discover actions defined in yaml.
package yamldiscovery

import (
	"github.com/spf13/cobra"

	"github.com/launchrctl/launchr/internal/launchr"
	"github.com/launchrctl/launchr/pkg/action"
	"github.com/launchrctl/launchr/pkg/cli"
)

func init() {
	launchr.RegisterPlugin(&Plugin{})
}

// Plugin is a plugin to discover actions defined in yaml.
type Plugin struct {
	app launchr.App
}

// PluginInfo implements launchr.Plugin interface.
func (p *Plugin) PluginInfo() launchr.PluginInfo {
	return launchr.PluginInfo{}
}

// OnAppInit implements launchr.Plugin interface to provide discovered actions.
func (p *Plugin) OnAppInit(app launchr.App) error {
	p.app = app
	return nil
}

// DiscoverActions implements launchr.ActionDiscoveryPlugin interface.
func (p *Plugin) DiscoverActions(fs launchr.ManagedFS) ([]*action.Action, error) {
	if fs, ok := fs.(action.DiscoveryFS); ok {
		return action.NewYamlDiscovery(fs).Discover()
	}
	return nil, nil
}

// CobraAddCommands implements launchr.CobraPlugin interface to provide discovered actions.
func (p *Plugin) CobraAddCommands(rootCmd *cobra.Command) error {
	var discoverCmd = &cobra.Command{
		Use:   "discover",
		Short: "Discovers available actions in filesystem",
		RunE: func(cmd *cobra.Command, args []string) error {
			var actions []*action.Action
			for _, fs := range p.app.GetRegisteredFS() {
				res, err := p.DiscoverActions(fs)
				if err != nil {
					return err
				}
				actions = append(actions, res...)
			}

			// @todo cache discovery to read fs only once.
			for _, a := range actions {
				cli.Println("%s", a.ID)
			}

			return nil
		},
	}
	// Discover actions.
	rootCmd.AddCommand(discoverCmd)
	return nil
}
