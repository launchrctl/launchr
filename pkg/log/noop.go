package log

import (
	"fmt"
	"os"
)

type noopLogger struct{}

// NewNoopLogger creates a new logger that doesn't do anything.
func NewNoopLogger() Logger {
	return &noopLogger{}
}

// Debug implements Logger.Debug.
func (l *noopLogger) Debug(string, ...any) {
}

// Info implements Logger.Info.
func (l *noopLogger) Info(string, ...any) {
}

// Warn implements Logger.Warn.
func (l *noopLogger) Warn(string, ...any) {
}

// Err implements Logger.Err.
func (l *noopLogger) Err(string, ...any) {
}

// Panic implements Logger.Panic.
func (l *noopLogger) Panic(format string, v ...any) {
	panic(fmt.Sprintf(format, v...))
}

// Fatal implements Logger.Fatal.
func (l *noopLogger) Fatal(string, ...any) {
	os.Exit(1)
}

// SetLevel implements Logger.SetLevel.
func (l *noopLogger) SetLevel(Level) {
}
