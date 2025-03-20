// Package launchr has application implementation.
package launchr

import (
	"io"
	"io/fs"

	"github.com/launchrctl/launchr/internal/launchr"
	"github.com/launchrctl/launchr/pkg/action"
)

// PkgPath is a main module path.
const PkgPath = launchr.PkgPath

// Log levels.
const (
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

// Application environment variables.
const (
	// EnvVarRootParentPID defines parent process id. May be used by forked processes.
	EnvVarRootParentPID = launchr.EnvVarRootParentPID
	// EnvVarActionsPath defines path where to search for actions.
	EnvVarActionsPath = launchr.EnvVarActionsPath
	// EnvVarLogLevel defines currently set log level.
	EnvVarLogLevel = launchr.EnvVarLogLevel
	// EnvVarLogFormat defines currently set log format, see --log-format flag.
	EnvVarLogFormat = launchr.EnvVarLogFormat
	// EnvVarQuietMode defines if the application should output anything, see --quiet flag.
	EnvVarQuietMode = launchr.EnvVarQuietMode
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

	// SensitiveMask replaces sensitive strings with a mask.
	SensitiveMask = launchr.SensitiveMask
	// MaskingWriter is a writer that masks sensitive data in the input stream.
	// It buffers data to handle cases where sensitive data spans across writes.
	MaskingWriter = launchr.MaskingWriter

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
	// PersistentPreRunPlugin is an interface to implement a plugin
	// to run before any command is run and all arguments are parsed.
	PersistentPreRunPlugin = launchr.PersistentPreRunPlugin
	// GeneratePlugin is an interface to generate supporting files before build.
	GeneratePlugin = launchr.GeneratePlugin
	// GenerateConfig defines generation config.
	GenerateConfig = launchr.GenerateConfig
	// PluginManager handles plugins.
	PluginManager = launchr.PluginManager
	// ServiceInfo provides service info for its initialization.
	ServiceInfo = launchr.ServiceInfo
	// Service is a common interface for a service to register.
	Service = launchr.Service
	// Config handles application configuration.
	Config = launchr.Config
	// ManagedFS is a File System managed by launchr.
	ManagedFS = launchr.ManagedFS

	// ExitError is an error holding an error code of executed command.
	ExitError = launchr.ExitError
	// EnvVar defines an environment variable and provides an interface to interact with it
	// by prefixing the current app name.
	// For example, if "my_var" is given as the variable name and the app name is "launchr",
	// the accessed environment variable will be "LAUNCHR_MY_VAR".
	EnvVar = launchr.EnvVar
)

// Version provides app version info.
func Version() *AppVersion { return launchr.Version() }

// RegisterPlugin add a plugin to global pull.
func RegisterPlugin(p Plugin) { launchr.RegisterPlugin(p) }

// FsRealpath returns absolute path for a [fs.FS] interface.
func FsRealpath(fs fs.FS) string { return launchr.FsRealpath(fs) }

// EnsurePath creates all directories in the path.
func EnsurePath(parts ...string) error { return launchr.EnsurePath(parts...) }

// Term returns default [Terminal] to print application messages to the console.
func Term() *Terminal { return launchr.Term() }

// NewIn returns a new [In] object from a [io.ReadCloser].
func NewIn(in io.ReadCloser) *In { return launchr.NewIn(in) }

// NewOut returns a new [Out] object from a [io.Writer].
func NewOut(out io.Writer) *Out { return launchr.NewOut(out) }

// MaskedStdStreams sets a cli in, out and err streams with the standard streams and with masking of sensitive data.
func MaskedStdStreams(mask *SensitiveMask) Streams { return launchr.MaskedStdStreams(mask) }

// NewBasicStreams creates streams with given in, out and err streams.
// Give decorate functions to extend functionality.
func NewBasicStreams(in io.ReadCloser, out io.Writer, err io.Writer, fns ...launchr.StreamsModifierFn) Streams {
	return launchr.NewBasicStreams(in, out, err, fns...)
}

// NoopStreams provides streams like /dev/null.
func NoopStreams() Streams { return launchr.NoopStreams() }

// StdInOutErr returns the standard streams (stdin, stdout, stderr).
//
// On Windows, it attempts to turn on VT handling on all std handles if
// supported, or falls back to terminal emulation. On Unix, this returns
// the standard [os.Stdin], [os.Stdout] and [os.Stderr].
func StdInOutErr() (stdIn io.ReadCloser, stdOut, stdErr io.Writer) { return launchr.StdInOutErr() }

// NewMaskingWriter initializes a new MaskingWriter.
func NewMaskingWriter(w io.Writer, mask *SensitiveMask) io.WriteCloser {
	return launchr.NewMaskingWriter(w, mask)
}

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

// NewExitError creates a new ExitError.
func NewExitError(code int, msg string) error { return launchr.NewExitError(code, msg) }

// RegisterCleanupFn saves a function to be executed on Cleanup.
// It is run on the termination of the application.
func RegisterCleanupFn(fn func() error) { launchr.RegisterCleanupFn(fn) }

// MustAbs returns absolute filepath and panics on error.
func MustAbs(path string) string { return launchr.MustAbs(path) }

// MustSubFS returns an [fs.FS] corresponding to the subtree rooted at fsys's dir.
func MustSubFS(fsys fs.FS, path string) fs.FS { return launchr.MustSubFS(fsys, path) }
