// Package yamldiscovery implements a launchr plugin to
// discover actions defined in yaml.
package yamldiscovery

import (
	"context"

	"github.com/launchrctl/launchr/internal/launchr"
	"github.com/launchrctl/launchr/pkg/action"
)

func init() {
	launchr.RegisterPlugin(Plugin{})
}

// Plugin is a plugin to discover actions defined in yaml.
type Plugin struct{}

// PluginInfo implements launchr.Plugin interface.
func (p Plugin) PluginInfo() launchr.PluginInfo {
	return launchr.PluginInfo{}
}

// OnAppInit implements launchr.Plugin interface to provide discovered actions.
func (p Plugin) OnAppInit(_ launchr.App) error {
	return nil
}

// DiscoverActions implements action.DiscoveryPlugin interface.
func (p Plugin) DiscoverActions(ctx context.Context, fs launchr.ManagedFS, idp action.IDProvider) ([]*action.Action, error) {
	if fs, ok := fs.(action.DiscoveryFS); ok {
		d := action.NewYamlDiscovery(fs)
		d.SetActionIDProvider(idp)
		return d.Discover(ctx)
	}
	return nil, nil
}
