// Package yamldiscovery implements a launchr plugin to
// discover actions defined in yaml.
package yamldiscovery

import (
	"os"
	"path/filepath"

	"github.com/launchrctl/launchr"
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
