// Package yamldiscovery implements a launchr plugin to
// discover actions defined in yaml.
package yamldiscovery

import (
	"os"
	"path/filepath"

	"github.com/launchrctl/launchr/core"
)

// ID is a plugin id.
const ID = "actions.yamldiscovery"

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

// GetDiscoveryPath provides actions yamldiscovery absolute path.
func GetDiscoveryPath() (string, error) {
	sp := os.Getenv("LAUNCHR_DISCOVERY_PATH")
	if sp == "" {
		sp = "./"
	}
	return filepath.Abs(sp)
}
