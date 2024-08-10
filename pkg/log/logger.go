// Package log is meant to provide a global logger for the application
// and provide logging functionality interface.
package log

import "time"

// Level defines a log level.
type Level int

// Log level constants.
const (
	DebugLvl Level = iota
	InfoLvl
	WarnLvl
	ErrLvl
	FatalLvl
)

// Config stores configuration for a logger.
type Config struct {
	Prefix    string
	Verbosity Level
}

// Logger interface defines leveled logging functionality.
type Logger interface {
	Debug(format string, v ...any)
	Info(format string, v ...any)
	Warn(format string, v ...any)
	Err(format string, v ...any)
	Panic(format string, v ...any)
	Fatal(format string, v ...any)
	SetLevel(Level)
}

var l = NewNoopLogger()

// SetGlobalLogger sets a package default global logger.
func SetGlobalLogger(newL Logger) {
	l = newL
}

// Debug runs Logger.Debug with a global logger.
func Debug(format string, v ...any) {
	l.Debug(format, v...)
}

// Info runs Logger.Info with a global logger.
func Info(format string, v ...any) {
	l.Info(format, v...)
}

// Warn runs Logger.Warn with a global logger.
func Warn(format string, v ...any) {
	l.Warn(format, v...)
}

// Err runs Logger.Err with a global logger.
func Err(format string, v ...any) {
	l.Err(format, v...)
}

// Panic runs Logger.Panic with a global logger.
func Panic(format string, v ...any) {
	l.Panic(format, v...)
}

// Fatal runs Logger.Fatal with a global logger.
func Fatal(format string, v ...any) {
	l.Fatal(format, v...)
}

// SetLevel runs Logger.SetLevel with a global logger.
func SetLevel(lvl Level) {
	l.SetLevel(lvl)
}

// DebugTimer returns a function that prints the name argument and
// the elapsed time between the call to timer and the call to
// the returned function. The returned function is intended to
// be used in a defer statement:
//
// defer DebugTimer("sum")().
func DebugTimer(name string) func() {
	start := time.Now()
	return func() {
		Debug("%s took %v\n", name, time.Since(start).Round(time.Millisecond))
	}
}
