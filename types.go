// Package launchr has application implementation.
package launchr

import (
	"io"
	"io/fs"

	"github.com/launchrctl/launchr/internal/launchr"
	"github.com/launchrctl/launchr/pkg/action"
)

const (
	// PkgPath is a main module path.
	PkgPath = launchr.PkgPath

	LogLevelDisabled = launchr.LogLevelDisabled // LogLevelDisabled does never print.
	LogLevelDebug    = launchr.LogLevelDebug    // LogLevelDebug is the log level for debug.
	LogLevelInfo     = launchr.LogLevelInfo     // LogLevelInfo is the log level for info.
	LogLevelWarn     = launchr.LogLevelWarn     // LogLevelWarn is the log level for warnings.
	LogLevelError    = launchr.LogLevelError    // LogLevelError is the log level for errors.
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
	// Command is an application command to execute.
	Command = launchr.Command
	// Template provides templating functionality to generate files.
	Template = launchr.Template

	// Logger is a logger and its config holder struct.
	Logger = launchr.Logger
	// LogOptions is a common interface to allow adjusting the logger.
	LogOptions = launchr.LogOptions
	// Slog is an alias for a go structured logger [slog.Logger] to reduce visible dependencies.
	Slog = launchr.Slog
	// A LogLevel is the importance or severity of a log event.
	LogLevel = launchr.LogLevel

	// Terminal prints formatted text to the console.
	Terminal = launchr.Terminal
	// TextPrinter contains methods to print formatted text to the console or return it as a string.
	TextPrinter = launchr.TextPrinter
	// Streams is an interface which exposes the standard input and output streams.
	Streams = launchr.Streams
	// In is an input stream used by the app to read user input.
	In = launchr.In
	// Out is an output stream used by the app to write normal program output.
	Out = launchr.Out

	// PluginInfo provides information about the plugin and is used as a unique data to indentify a plugin.
	PluginInfo = launchr.PluginInfo
	// Plugin is a common interface for launchr plugins.
	Plugin = launchr.Plugin
	// OnAppInitPlugin is an interface to implement a plugin for app initialisation.
	OnAppInitPlugin = launchr.OnAppInitPlugin
	// ActionDiscoveryPlugin is an interface to implement a plugin to discover actions.
	ActionDiscoveryPlugin = action.DiscoveryPlugin
	// ActionsAlterPlugin is in interface to implement a plugin to alter registered actions.
	ActionsAlterPlugin = action.AlterActionsPlugin
	// CobraPlugin is an interface to implement a plugin for cobra.
	CobraPlugin = launchr.CobraPlugin
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
	// ManagedFS is a File System managed by launchr.
	ManagedFS = launchr.ManagedFS
)

// Version provides app version info.
func Version() *AppVersion { return launchr.Version() }

// RegisterPlugin add a plugin to global pull.
func RegisterPlugin(p Plugin) { launchr.RegisterPlugin(p) }

// GetFsAbsPath returns absolute path for an FS struct.
func GetFsAbsPath(fs fs.FS) string { return launchr.GetFsAbsPath(fs) }

// EnsurePath creates all directories in the path.
func EnsurePath(parts ...string) error { return launchr.EnsurePath(parts...) }

// Term returns default [Terminal] to print application messages to the console.
func Term() *Terminal { return launchr.Term() }

// NewIn returns a new [In] object from a [io.ReadCloser].
func NewIn(in io.ReadCloser) *In { return launchr.NewIn(in) }

// NewOut returns a new [Out] object from a [io.Writer].
func NewOut(out io.Writer) *Out { return launchr.NewOut(out) }

// StandardStreams sets a cli in, out and err streams with the standard streams.
func StandardStreams() Streams { return launchr.StandardStreams() }

// NoopStreams provides streams like /dev/null.
func NoopStreams() Streams { return launchr.NoopStreams() }

// Log returns the default logger.
func Log() *Logger { return launchr.Log() }

// SetLogger sets the default logger.
func SetLogger(l *Logger) { launchr.SetLogger(l) }

// NewConsoleLogger creates a default console logger.
func NewConsoleLogger(w io.Writer) *Logger { return launchr.NewConsoleLogger(w) }

// NewTextHandlerLogger creates a logger with a [io.Writer] and plain output.
func NewTextHandlerLogger(w io.Writer) *Logger { return launchr.NewTextHandlerLogger(w) }

// NewJSONHandlerLogger creates a logger with a [io.Writer] and JSON output.
func NewJSONHandlerLogger(w io.Writer) *Logger { return launchr.NewJSONHandlerLogger(w) }
