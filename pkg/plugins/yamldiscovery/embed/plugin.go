package embed

import (
	"github.com/launchrctl/launchr"
)

// ID is a plugin id.
const ID = "actions.yamldiscovery.embed"

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
	return nil
}
