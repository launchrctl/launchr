package launchr

import (
	"fmt"
	"io"
	"log/slog"
	"sync/atomic"

	"github.com/pterm/pterm"
)

var defaultLogger atomic.Pointer[Logger]

func init() {
	// By default, don't output any logs.
	SetLogger(NewTextHandlerLogger(io.Discard))
}

// Slog is an alias for a go structured logger [slog.Logger] to reduce visible dependencies.
type Slog = slog.Logger

// Logger is a logger and its config holder struct.
type Logger struct {
	*Slog
	LogOptions
}

// A LogLevel is the importance or severity of a log event.
type LogLevel int

// Log levels.
const (
	LogLevelDisabled LogLevel = iota // LogLevelDisabled does never print.
	LogLevelDebug                    // LogLevelDebug is the log level for debug.
	LogLevelInfo                     // LogLevelInfo is the log level for info.
	LogLevelWarn                     // LogLevelWarn is the log level for warnings.
	LogLevelError                    // LogLevelError is the log level for errors.
)

// LogLevel string constants.
const (
	LogLevelStrDisabled string = "NONE"  // LogLevelStrDisabled does never print.
	LogLevelStrDebug    string = "DEBUG" // LogLevelStrDebug is the log level for debug.
	LogLevelStrInfo     string = "INFO"  // LogLevelStrInfo is the log level for info.
	LogLevelStrWarn     string = "WARN"  // LogLevelStrWarn is the log level for warnings.
	LogLevelStrError    string = "ERROR" // LogLevelStrError is the log level for errors.
)

// String implements [fmt.Stringer] interface.
func (l LogLevel) String() string {
	switch l {
	case LogLevelDebug:
		return LogLevelStrDebug
	case LogLevelInfo:
		return LogLevelStrInfo
	case LogLevelWarn:
		return LogLevelStrWarn
	case LogLevelError:
		return LogLevelStrError
	default:
		return LogLevelStrDisabled
	}
}

// LogLevelFromString translates a log level string to
func LogLevelFromString(s string) LogLevel {
	switch s {
	case LogLevelStrDisabled:
		return LogLevelDisabled
	case LogLevelStrError:
		return LogLevelError
	case LogLevelStrWarn:
		return LogLevelWarn
	case LogLevelStrInfo:
		return LogLevelInfo
	case LogLevelStrDebug:
		return LogLevelDebug
	default:
		return LogLevelDisabled
	}
}

// LogOptions is a common interface to allow adjusting the logger.
type LogOptions interface {
	// Level returns the currently set log level.
	Level() LogLevel
	// SetLevel sets log level.
	SetLevel(l LogLevel)
	// SetOutput sets logger output.
	SetOutput(w io.Writer)
}

type ptermOpts struct {
	pterm *pterm.Logger
	lvl   LogLevel
}

func (o *ptermOpts) Level() LogLevel {
	return o.lvl
}

func (o *ptermOpts) SetLevel(l LogLevel) {
	o.lvl = l
	o.pterm.Level = o.mapLevel(l)
}

func (o *ptermOpts) SetOutput(w io.Writer) {
	o.pterm.Writer = w
}

func (o *ptermOpts) mapLevel(l LogLevel) pterm.LogLevel {
	switch l {
	case LogLevelDisabled:
		return pterm.LogLevelDisabled
	case LogLevelDebug:
		return pterm.LogLevelDebug
	case LogLevelInfo:
		return pterm.LogLevelInfo
	case LogLevelWarn:
		return pterm.LogLevelWarn
	case LogLevelError:
		return pterm.LogLevelError
	default:
		panic(fmt.Sprintf("mapping for LogLevel(%d) is missing for pterm", l))
	}
}

type slogOpts struct {
	io.Writer      // it is used to allow runtime changes, which slog does not support.
	*slog.LevelVar // slog handler level for runtime changes.
	LogLevel       // current launchr level.
}

func (o *slogOpts) Level() LogLevel {
	return o.LogLevel
}

func (o *slogOpts) SetLevel(l LogLevel) {
	o.LogLevel = l
	o.LevelVar.Set(o.mapLevel(l))
}

func (o *slogOpts) SetOutput(w io.Writer) {
	o.Writer = w
}

func (o *slogOpts) mapLevel(l LogLevel) slog.Level {
	switch l {
	case LogLevelDisabled:
		// Return super high level to discard all logs.
		return slog.Level(100)
	case LogLevelDebug:
		return slog.LevelDebug
	case LogLevelInfo:
		return slog.LevelInfo
	case LogLevelWarn:
		return slog.LevelWarn
	case LogLevelError:
		return slog.LevelError
	default:
		panic(fmt.Sprintf("mapping for LogLevel(%d) is missing for slog", l))
	}
}

// NewConsoleLogger creates a default console logger.
func NewConsoleLogger(w io.Writer) *Logger {
	l := pterm.DefaultLogger
	opts := &ptermOpts{pterm: &l}
	opts.SetOutput(w)
	return &Logger{
		Slog:       slog.New(pterm.NewSlogHandler(opts.pterm)),
		LogOptions: opts,
	}
}

func newSlogOpts(w io.Writer) (*slogOpts, *slog.HandlerOptions) {
	opts := &slogOpts{Writer: w, LevelVar: &slog.LevelVar{}}
	handlerOpts := &slog.HandlerOptions{Level: opts.LevelVar}
	return opts, handlerOpts
}

// NewTextHandlerLogger creates a logger with a [io.Writer] and plain slog output.
func NewTextHandlerLogger(w io.Writer) *Logger {
	opts, handlerOpts := newSlogOpts(w)
	return &Logger{
		Slog:       slog.New(slog.NewTextHandler(opts, handlerOpts)),
		LogOptions: opts,
	}
}

// NewJSONHandlerLogger creates a logger with a [io.Writer] and JSON output.
func NewJSONHandlerLogger(w io.Writer) *Logger {
	opts, handlerOpts := newSlogOpts(w)
	return &Logger{
		Slog:       slog.New(slog.NewJSONHandler(opts, handlerOpts)),
		LogOptions: opts,
	}
}

// Log returns the default logger.
func Log() *Logger {
	return defaultLogger.Load()
}

// SetLogger sets the default logger.
func SetLogger(l *Logger) {
	defaultLogger.Store(l)
}
