// Package log is meant to provide a global logger for the application
// and provide logging functionality interface.
package log

import (
	"fmt"
	"strconv"

	"github.com/launchrctl/launchr/internal/launchr"
)

func convertToSlogArgs(args ...any) []any {
	if args == nil {
		return nil
	}
	res := make([]any, 2*len(args))
	for i, v := range args {
		res[2*i] = "arg" + strconv.Itoa(i)
		res[2*i+1] = v
	}
	return res
}

// Debug runs Logger.Debug with a global logger.
// Deprecated: use the new structured logger - `launchr.Log().Debug(msg, argName1, argVal1, argName2, ...)`
func Debug(format string, v ...any) {
	launchr.Log().Debug(format, convertToSlogArgs(v)...)
}

// Info runs Logger.Info with a global logger.
// Deprecated: use the new structured logger - `launchr.Log().Info(msg, argName1, argVal1, argName2, ...)`
func Info(format string, v ...any) {
	launchr.Log().Info(format, convertToSlogArgs(v)...)
}

// Warn runs Logger.Warn with a global logger.
// Deprecated: use the new structured logger - `launchr.Log().Warn(msg, argName1, argVal1, argName2, ...)`
func Warn(format string, v ...any) {
	launchr.Log().Warn(format, convertToSlogArgs(v)...)
}

// Err runs Logger.Err with a global logger.
// Deprecated: use the new structured logger - `launchr.Log().Error(msg, argName1, argVal1, argName2, ...)`
func Err(format string, v ...any) {
	Error(format, v...)
}

// Error runs Logger.Err with a global logger.
// Deprecated: use new structured logger - `launchr.Log().Error(msg, argName1, argVal1, argName2, ...)`
func Error(format string, v ...any) {
	launchr.Log().Error(format, convertToSlogArgs(v)...)
}

// Panic runs Logger.Panic with a global logger.
// Deprecated: no longer supported with no direct replacement.
func Panic(format string, v ...any) { //nolint:revive
	panic(fmt.Sprint("DEPRECATED log.Panic call.", format))
}

// Fatal runs Logger.Fatal with a global logger.
// Deprecated: no longer supported with no direct replacement.
func Fatal(format string, v ...any) { //nolint:revive
	panic(fmt.Sprint("DEPRECATED log.Fatal call.", format))
}
