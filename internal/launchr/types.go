package launchr

import (
	"fmt"
	"io/fs"

	"github.com/spf13/cobra"

	"github.com/launchrctl/launchr/pkg/cli"
	"github.com/launchrctl/launchr/pkg/jsonschema"
)

// PkgPath is a main module path.
const PkgPath = "github.com/launchrctl/launchr"

// App stores global application state.
type App interface {
	// Name returns app name.
	Name() string
	// GetWD provides app's working dir.
	GetWD() string
	// Streams returns application cli.
	Streams() cli.Streams
	// AddService registers a service in the app.
	// Panics if a service is not unique.
	AddService(s Service)
	// GetService retrieves a service of type v and assigns it to v.
	// Panics if a service is not found.
	GetService(v interface{})
	// RegisterFS registers a File System in launchr.
	// It may be a FS for action discovery, see action.DiscoveryFS.
	RegisterFS(fs ManagedFS)
	// GetRegisteredFS returns an array of registered File Systems.
	GetRegisteredFS() []ManagedFS
}

// AppVersion stores application version.
type AppVersion struct {
	Name        string
	Version     string
	OS          string
	Arch        string
	BuiltWith   string
	CoreVersion string
	CoreReplace string
	Plugins     []string
}

// PluginInfo provides information about the plugin and is used as a unique data to indentify a plugin.
type PluginInfo struct {
	// Weight defines the order of plugins calling. @todo rework to a real dependency resolving.
	Weight   int
	pkgPath  string
	typeName string
}

func (p PluginInfo) String() string {
	return p.pkgPath + "." + p.typeName
}

// InitPluginInfo sets private fields for internal usage only.
func InitPluginInfo(pi *PluginInfo, p Plugin) {
	pi.pkgPath, pi.typeName = GetTypePkgPathName(p)
}

// PluginsMap is a type alias for plugins map.
type PluginsMap = map[PluginInfo]Plugin

// Plugin is a common interface for launchr plugins.
type Plugin interface {
	// PluginInfo requests a type to provide information about the plugin.
	PluginInfo() PluginInfo
}

// OnAppInitPlugin is an interface to implement a plugin for app initialisation.
type OnAppInitPlugin interface {
	Plugin
	// OnAppInit is hook function called on application initialisation.
	// Plugins may save app global object, retrieve or provide services here.
	OnAppInit(app App) error
}

// CobraPlugin is an interface to implement a plugin for cobra.
type CobraPlugin interface {
	Plugin
	// CobraAddCommands is a hook called when cobra root command is available.
	// Plugins may register its command line commands here.
	CobraAddCommands(*cobra.Command) error
}

// PluginGeneratedData is a struct containing a result information of plugin generation.
type PluginGeneratedData struct {
	Plugins []string
}

// GeneratePlugin is an interface to generate supporting files before build.
type GeneratePlugin interface {
	Plugin
	// Generate is a function called when application is generating code and assets for the build.
	Generate(buildPath string, workDir string) (*PluginGeneratedData, error)
}

// registeredPlugins is a store for plugins on init.
var registeredPlugins = make(PluginsMap)

// RegisterPlugin add a plugin to global pull.
func RegisterPlugin(p Plugin) {
	info := p.PluginInfo()
	InitPluginInfo(&info, p)
	if _, ok := registeredPlugins[info]; ok {
		panic(fmt.Errorf("plugin %q already registered, please, review the build", info))
	}
	registeredPlugins[info] = p
}

// PluginManager handles plugins.
type PluginManager interface {
	Service
	All() PluginsMap
}

// NewPluginManagerWithRegistered creates PluginManager with registered plugins.
func NewPluginManagerWithRegistered() PluginManager {
	return pluginManagerMap(registeredPlugins)
}

type pluginManagerMap PluginsMap

func (m pluginManagerMap) ServiceInfo() ServiceInfo { return ServiceInfo{} }
func (m pluginManagerMap) All() PluginsMap          { return m }

// ServiceInfo provides service info for its initialization.
type ServiceInfo struct {
	pkgPath  string
	typeName string
}

func (s ServiceInfo) String() string {
	return s.pkgPath + "." + s.typeName
}

// Service is a common interface for a service to register.
type Service interface {
	ServiceInfo() ServiceInfo
}

// InitServiceInfo sets private fields for internal usage only.
func InitServiceInfo(si *ServiceInfo, s Service) {
	si.pkgPath, si.typeName = GetTypePkgPathName(s)
}

// ManagedFS is a common interface for FS registered in launchr.
type ManagedFS interface {
	fs.FS
	FS() fs.FS
}

// ValueProcessor defines an interface for processing a value based on its type and some options.
type ValueProcessor interface {
	IsApplicable(valueType jsonschema.Type) bool
	Execute(value interface{}, options map[string]interface{}) (interface{}, error)
}

// ValueProcessorFn is a function signature used as a callback in processors.
type ValueProcessorFn func(value interface{}, options map[string]interface{}) (interface{}, error)
