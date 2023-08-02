package launchr

import (
	"fmt"
	"reflect"

	"github.com/launchrctl/launchr/pkg/action"
)

// ServiceInfo provides service info for its initialization.
type ServiceInfo struct{}

// Service is a common interface for a service to register.
type Service interface {
	ServiceInfo() ServiceInfo
}

// GetService returns a service from the app. It uses generics to find the exact service.
func GetService[T Service](app *App) T {
	services := app.serviceMngr.All()
	for _, s := range services {
		s, ok := s.(T)
		if ok {
			return s
		}
	}
	panic(fmt.Sprintf("service %s does not exist", reflect.TypeOf((*T)(nil)).Elem()))
}

// ActionManagerID is an action manager service id.
const ActionManagerID = "action_manager"

// ActionManager handles actions.
type ActionManager interface {
	Service
	Add(*action.Command)
	All() map[string]*action.Command
}

type actionManagerMap map[string]*action.Command

func newActionManager() ActionManager {
	return make(actionManagerMap)
}

func (m actionManagerMap) ServiceInfo() ServiceInfo {
	return ServiceInfo{}
}

func (m actionManagerMap) Add(cmd *action.Command) {
	m[cmd.CommandName] = cmd
}

func (m actionManagerMap) All() map[string]*action.Command {
	return m
}

// PluginManagerID is a plugin manager service id.
const PluginManagerID = "plugin_manager"

// PluginManager handles plugins.
type PluginManager interface {
	Service
	All() map[PluginInfo]Plugin
}

type pluginManagerMap map[PluginInfo]Plugin

func (m pluginManagerMap) ServiceInfo() ServiceInfo {
	return ServiceInfo{}
}

func (m pluginManagerMap) All() map[PluginInfo]Plugin {
	return m
}

// GetPluginByType returns specific plugins from the app.
func GetPluginByType[T Plugin](app *App) []T {
	plugins := app.pluginMngr.All()
	res := make([]T, 0, len(plugins))
	for _, p := range plugins {
		p, ok := p.(T)
		if ok {
			res = append(res, p)
		}
	}
	return res
}

// ServiceManagerID is a service manager service id.
const ServiceManagerID = "service_manager"

// ServiceManager handles services.
type ServiceManager interface {
	Service
	Add(name string, s Service)
	All() map[string]Service
}

type serviceManagerMap map[string]Service

func newServiceManager() ServiceManager {
	return make(serviceManagerMap)
}

func (m serviceManagerMap) ServiceInfo() ServiceInfo {
	return ServiceInfo{}
}

func (m serviceManagerMap) Add(name string, s Service) {
	m[name] = s
}

func (m serviceManagerMap) All() map[string]Service {
	return m
}
