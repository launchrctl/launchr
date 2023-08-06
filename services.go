package launchr

import (
	"fmt"
	"reflect"

	"github.com/launchrctl/launchr/internal/launchr"
	"github.com/launchrctl/launchr/internal/launchr/config"
)

// GetService returns a service from the app. It uses generics to find the exact service.
func GetService[T Service](app *App) T {
	for _, s := range app.services {
		s, ok := s.(T)
		if ok {
			return s
		}
	}
	panic(fmt.Sprintf("service %s does not exist", reflect.TypeOf((*T)(nil)).Elem()))
}

type (
	// ServiceInfo provides service info for its initialization.
	ServiceInfo = launchr.ServiceInfo
	// Service is a common interface for a service to register.
	Service = launchr.Service
	// GlobalConfig handles global configuration.
	GlobalConfig = config.GlobalConfig
	// GlobalConfigAware provides an interface for structs to support global configuration setting.
	GlobalConfigAware = config.GlobalConfigAware
)

// PluginManager handles plugins.
type PluginManager interface {
	Service
	All() map[PluginInfo]Plugin
}

type pluginManagerMap map[PluginInfo]Plugin

func (m pluginManagerMap) ServiceInfo() ServiceInfo   { return ServiceInfo{ID: "plugin_manager"} }
func (m pluginManagerMap) All() map[PluginInfo]Plugin { return m }
