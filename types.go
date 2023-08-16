// Package launchr has application implementation.
package launchr

import (
	"io/fs"

	"github.com/launchrctl/launchr/internal/launchr"
)

const (
	// PkgPath is a main module path.
	PkgPath = launchr.PkgPath
)

// Variables for version provided by ldflags.
var (
	name      = "launchr"
	version   = "dev"
	builtWith string //nolint:unused
)

// Re-export types aliases for usage by external modules.
type (
	// App stores global application state.
	App = launchr.App
	// AppVersion stores application version.
	AppVersion = launchr.AppVersion
	// PluginInfo provides information about the plugin and is used as a unique data to indentify a plugin.
	PluginInfo = launchr.PluginInfo
	// Plugin is a common interface for launchr plugins.
	Plugin = launchr.Plugin
	// OnAppInitPlugin is an interface to implement a plugin for app initialisation.
	OnAppInitPlugin = launchr.OnAppInitPlugin
	// CobraPlugin is an interface to implement a plugin for cobra.
	CobraPlugin = launchr.CobraPlugin
	// PluginGeneratedData is a struct containing a result information of plugin generation.
	PluginGeneratedData = launchr.PluginGeneratedData
	// GeneratePlugin is an interface to generate supporting files before build.
	GeneratePlugin = launchr.GeneratePlugin
	// PluginManager handles plugins.
	PluginManager = launchr.PluginManager
	// ServiceInfo provides service info for its initialization.
	ServiceInfo = launchr.ServiceInfo
	// Service is a common interface for a service to register.
	Service = launchr.Service
	// Config handles application configuration.
	Config = launchr.Config
	// ConfigAware provides an interface for structs to support launchr configuration setting.
	ConfigAware = launchr.ConfigAware
)

// Version provides app version info.
func Version() *AppVersion { return launchr.Version() }

// RegisterPlugin add a plugin to global pull.
func RegisterPlugin(p Plugin) { launchr.RegisterPlugin(p) }

// GetFsAbsPath returns absolute path for an FS struct.
func GetFsAbsPath(fs fs.FS) string { return launchr.GetFsAbsPath(fs) }

// EnsurePath creates all directories in the path.
func EnsurePath(parts ...string) error { return launchr.EnsurePath(parts...) }

// ToCamelCase converts a string to CamelCase.
func ToCamelCase(s string, capFirst bool) string { return launchr.ToCamelCase(s, capFirst) }
