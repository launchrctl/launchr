// Package testactions contains actions that help to test the app.
package testactions

import (
	"context"

	"github.com/launchrctl/launchr/internal/launchr"
	"github.com/launchrctl/launchr/pkg/action"
)

func init() {
	launchr.RegisterPlugin(&Plugin{})
}

// Plugin is a test plugin declaration.
type Plugin struct {
	app launchr.App
}

// PluginInfo implements [launchr.Plugin] interface.
func (p *Plugin) PluginInfo() launchr.PluginInfo {
	return launchr.PluginInfo{}
}

// OnAppInit implements [launchr.OnAppInitPlugin] interface.
func (p *Plugin) OnAppInit(app launchr.App) error {
	p.app = app
	var am action.Manager
	app.Services().Get(&am)
	// Add custom fs to default discovery.
	app.RegisterFS(action.NewDiscoveryFS(registeredEmbedFS, app.GetWD()))
	// Create a special decorator to output given input.
	am.AddDecorators(pluginPrintInput)
	return nil
}

// DiscoverActions implements [launchr.ActionDiscoveryPlugin] interface.
func (p *Plugin) DiscoverActions(_ context.Context) ([]*action.Action, error) {
	var mask *launchr.SensitiveMask
	p.app.Services().Get(&mask)
	return []*action.Action{
		actionSensitive(p.app, mask),
		actionLogLevels(),
		embedContainerAction(),
	}, nil
}
