package launchr

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"text/template"

	"github.com/spf13/cobra"
)

// PkgPath is a main module path.
const PkgPath = "github.com/launchrctl/launchr"

// Command is a type alias for [cobra.Command].
// to reduce direct dependency on cobra in packages.
type Command = cobra.Command

// CommandGroup is a type alias for [cobra.Group].
type CommandGroup = cobra.Group

// App stores global application state.
type App interface {
	// Name returns app name.
	Name() string
	// GetWD provides app's working dir.
	GetWD() string
	// Streams returns application cli.
	Streams() Streams
	// SetStreams sets application streams.
	SetStreams(s Streams)
	// AddService registers a service in the app.
	// Panics if a service is not unique.
	AddService(s Service)
	// GetService retrieves a service of type [v] and assigns it to [v].
	// Panics if a service is not found.
	GetService(v any)

	// RegisterFS registers a File System in launchr.
	// It may be a FS for action discovery, see [action.DiscoveryFS].
	RegisterFS(fs ManagedFS)
	// GetRegisteredFS returns an array of registered File Systems.
	GetRegisteredFS() []ManagedFS
}

// AppInternal is an extension to access cobra related functionality of the app.
// It is intended for internal use only to prevent coupling on volatile functionality.
type AppInternal interface {
	App
	RootCmd() *Command
	CmdEarlyParsed() CmdEarlyParsed
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

// PluginInfo provides information about the plugin and is used as a unique data to identify a plugin.
type PluginInfo struct {
	// Weight defines the order of plugins calling. @todo rework to a real dependency resolving.
	Weight   int
	pkgPath  string
	typeName string
}

func (p PluginInfo) String() string {
	return p.pkgPath + "." + p.typeName
}

// GetPackagePath returns the package path of the [PluginInfo].
func (p PluginInfo) GetPackagePath() string {
	return p.pkgPath
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
	CobraAddCommands(root *Command) error
}

// Template provides templating functionality to generate files.
type Template struct {
	Tmpl string // Tmpl is a template string.
	Data any    // Data is a template data.
}

// Generate executes a template and writes it.
func (t Template) Generate(w io.Writer) error {
	var tmpl = template.Must(template.New("tmp").Parse(t.Tmpl))
	return tmpl.Execute(w, t.Data)
}

// WriteFile creates/overwrites a file and executes the template with it.
func (t Template) WriteFile(name string) error {
	err := EnsurePath(filepath.Dir(name))
	if err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Clean(name), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	err = t.Generate(f)
	return err
}

// GeneratePlugin is an interface to generate supporting files before build.
type GeneratePlugin interface {
	Plugin
	// Generate is a function called when application is generating code and assets for the build.
	Generate(config GenerateConfig) error
}

// GenerateConfig defines generation config.
type GenerateConfig struct {
	WorkDir  string // WorkDir is where the script must consider current working directory.
	BuildDir string // BuildDir is where the script will output the result.
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

// NewPluginManagerWithRegistered creates [PluginManager] with registered plugins.
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

// MapItem is a helper struct used to return an ordered map as a slice.
type MapItem[K, V any] struct {
	K K // K is a key of the map item.
	V V // V is a value of the map item.
}

// ExitError is an error holding an error code of executed command.
type ExitError struct {
	code int
	msg  string
}

// NewExitError creates a new ExitError.
func NewExitError(code int, msg string) error {
	return ExitError{code, msg}
}

// Error implements error interface.
func (e ExitError) Error() string {
	return e.msg
}

// ExitCode returns the exit code.
func (e ExitError) ExitCode() int {
	return e.code
}
