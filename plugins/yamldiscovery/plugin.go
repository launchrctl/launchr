// Package yamldiscovery implements a launchr plugin to
// discover actions defined in yaml.
package yamldiscovery

import (
	"context"
	"math"

	"github.com/launchrctl/launchr/internal/launchr"
	"github.com/launchrctl/launchr/pkg/action"
)

func init() {
	launchr.RegisterPlugin(&Plugin{})
}

// Plugin is a [launchr.Plugin] to discover actions defined in yaml.
type Plugin struct {
	am  action.Manager
	app launchr.App
}

// PluginInfo implements [launchr.Plugin] interface.
func (p *Plugin) PluginInfo() launchr.PluginInfo {
	return launchr.PluginInfo{
		// Discover yaml files in fs as late as possible
		// to allow early return in discovery if the desired command is found.
		// Set the weight less than actionscobra plugin to run OnAppInit earlier.
		// @todo review after introducing dependency graph.
		Weight: math.MaxInt - 10,
	}
}

// OnAppInit implements [launchr.Plugin] interface to provide discovered actions.
func (p *Plugin) OnAppInit(app launchr.App) error {
	app.Services().Get(&p.am)
	p.app = app
	return nil
}

// DiscoverActions implements [action.DiscoveryPlugin] interface.
func (p *Plugin) DiscoverActions(ctx context.Context) ([]*action.Action, error) {
	var res []*action.Action
	idp := p.am.GetActionIDProvider()
	for _, regfs := range p.app.GetRegisteredFS() {
		if regfs, ok := regfs.(action.DiscoveryFS); ok {
			d := action.NewYamlDiscovery(regfs)
			d.SetActionIDProvider(idp)
			discovered, err := d.Discover(ctx)
			if err != nil {
				return nil, err
			}
			res = append(res, discovered...)
		}
	}

	return res, nil
}
