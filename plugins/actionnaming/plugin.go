// Package actionnaming is a plugin of launchr to adjust action ids.
package actionnaming

import (
	"strings"

	"github.com/launchrctl/launchr/internal/launchr"
	"github.com/launchrctl/launchr/pkg/action"
)

type launchrCfg struct {
	ActionsNaming []actionsNaming `yaml:"actions_naming"`
}

type actionsNaming struct {
	Search  string `yaml:"search"`
	Replace string `yaml:"replace"`
}

func init() {
	launchr.RegisterPlugin(Plugin{})
}

// Plugin is [launchr.Plugin] to improve actions naming.
type Plugin struct{}

// PluginInfo implements [launchr.Plugin] interface.
func (p Plugin) PluginInfo() launchr.PluginInfo {
	return launchr.PluginInfo{}
}

// OnAppInit implements [launchr.Plugin] interface.
func (p Plugin) OnAppInit(app launchr.App) error {
	// Get services.
	var cfg launchr.Config
	var am action.Manager
	app.GetService(&cfg)
	app.GetService(&am)

	// Load naming configuration.
	var launchrConfig launchrCfg
	// @todo refactor yaml property position.
	err := cfg.Get(launchr.ConfigKey, &launchrConfig)
	if err != nil {
		return err
	}
	// Override action id provider.
	if len(launchrConfig.ActionsNaming) == 0 {
		// No need to override.
		return nil
	}
	am.SetActionIDProvider(&ConfigActionIDProvider{
		parent: am.GetActionIDProvider(),
		naming: launchrConfig.ActionsNaming,
	})
	return nil
}

// ConfigActionIDProvider is an ID provider based on [launchr.Config] global configuration.
type ConfigActionIDProvider struct {
	parent action.IDProvider
	naming []actionsNaming
}

// GetID implements [action.IDProvider] interface.
func (idp *ConfigActionIDProvider) GetID(a *action.Action) string {
	id := idp.parent.GetID(a)
	newID := id
	for _, an := range idp.naming {
		newID = strings.ReplaceAll(newID, an.Search, an.Replace)
	}
	return newID
}
