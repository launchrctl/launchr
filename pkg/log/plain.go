package log

import (
	"log"
	"os"
)

type consoleLogger struct {
	out *log.Logger
	err *log.Logger
	Config
}

// NewPlainLogger creates a CLI plain text logger.
func NewPlainLogger(out *os.File, err *os.File, cfg *Config) Logger {
	if cfg == nil {
		cfg = &Config{
			"",
			InfoLvl,
		}
	}
	return &consoleLogger{
		out:    log.New(out, "", 0),
		err:    log.New(err, "", 0),
		Config: *cfg,
	}
}

// Debug implements Logger.Debug.
func (l *consoleLogger) Debug(format string, v ...any) {
	if l.Verbosity <= DebugLvl {
		l.out.Printf("DEBUG: "+format, v...)
	}
}

// Info implements Logger.Info.
func (l *consoleLogger) Info(format string, v ...any) {
	if l.Verbosity <= InfoLvl {
		l.out.Printf("INFO: "+format, v...)
	}
}

// Warn implements Logger.Warn.
func (l *consoleLogger) Warn(format string, v ...any) {
	if l.Verbosity <= WarnLvl {
		l.out.Printf("WARN: "+format, v...)
	}
}

// Err implements Logger.Err.
func (l *consoleLogger) Err(format string, v ...any) {
	if l.Verbosity <= ErrLvl {
		l.err.Printf("ERROR: "+format, v...) //nolint goconst
	}
}

// Panic implements Logger.Panic.
func (l *consoleLogger) Panic(format string, v ...any) {
	l.err.Panicf("ERROR: "+format, v...) //nolint goconst
}

// Fatal implements Logger.Fatal.
func (l *consoleLogger) Fatal(format string, v ...any) {
	l.err.Fatalf("ERROR: "+format, v...) //nolint goconst
}

// SetLevel implements Logger.SetLevel.
func (l *consoleLogger) SetLevel(lvl Level) {
	l.Verbosity = lvl
}
