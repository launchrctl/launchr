package embed

import (
	"github.com/launchrctl/launchr/core"
)

// ID is a plugin id.
const ID = "actions.yamldiscovery.embed"

func init() {
	core.RegisterPlugin(&Plugin{})
}

// Plugin is a plugin to discover actions defined in yaml.
type Plugin struct {
	app *core.App
}

// PluginInfo implements core.Plugin interface.
func (p *Plugin) PluginInfo() core.PluginInfo {
	return core.PluginInfo{
		ID: ID,
	}
}

// InitApp implements core.Plugin interface to provide discovered actions.
func (p *Plugin) InitApp(app *core.App) error {
	p.app = app
	return nil
}
