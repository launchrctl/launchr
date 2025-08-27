package launchr

import (
	"os"
	"strings"
	"syscall"
)

// Application environment variables.
const (
	// EnvVarRootParentPID defines parent process id. May be used by forked processes.
	EnvVarRootParentPID = EnvVar("root_ppid")
	// EnvVarActionsPath defines path where to search for actions.
	EnvVarActionsPath = EnvVar("actions_path")
	// EnvVarLogLevel defines currently set log level, see --log-level or -v flag.
	EnvVarLogLevel = EnvVar("log_level")
	// EnvVarLogFormat defines currently set log format, see --log-format flag.
	EnvVarLogFormat = EnvVar("log_format")
	// EnvVarQuietMode defines if the application should output anything, see --quiet flag.
	EnvVarQuietMode = EnvVar("quiet_mode")
)

// EnvVar defines an environment variable and provides an interface to interact with it
// by prefixing the current app name.
// For example, if "my_var" is given as the variable name and the app name is "launchr",
// the accessed environment variable will be "LAUNCHR_MY_VAR".
type EnvVar string

// String implements [fmt.Stringer] interface.
func (key EnvVar) String() string {
	return strings.ToUpper(name + "_" + string(key))
}

// EnvString returns an os string of env variable with a value val.
func (key EnvVar) EnvString(val string) string {
	return key.String() + "=" + val
}

// Get returns env variable value.
func (key EnvVar) Get() string {
	return os.Getenv(key.String())
}

// Set sets env variable.
func (key EnvVar) Set(val string) error {
	return os.Setenv(key.String(), val)
}

// Unset unsets env variable.
func (key EnvVar) Unset() error {
	return os.Unsetenv(key.String())
}

// Getenv is an environment variable expand callback.
func Getenv(key string) string {
	if key == "$" {
		return "$"
	}

	// Condition functions for expansion patterns
	var (
		varExists         = func(v string, exists bool) bool { return exists }
		varExistsNotEmpty = func(v string, exists bool) bool { return exists && v != "" }
	)

	// Handle shell parameter expansion patterns
	if idx := strings.Index(key, ":-"); idx != -1 {
		// ${var:-default} - use default if variable doesn't exist or is empty
		return envHandleExpansion(key[:idx], key[idx+2:], varExistsNotEmpty, true)
	}
	if idx := strings.Index(key, "-"); idx != -1 {
		// ${var-default} - use default if variable doesn't exist
		return envHandleExpansion(key[:idx], key[idx+1:], varExists, true)
	}
	if idx := strings.Index(key, ":+"); idx != -1 {
		// ${var:+alternative} - use alternative if variable exists and is not empty
		return envHandleExpansion(key[:idx], key[idx+2:], varExistsNotEmpty, false)
	}
	if idx := strings.Index(key, "+"); idx != -1 {
		// ${var+alternative} - use alternative if variable exists
		return envHandleExpansion(key[:idx], key[idx+1:], varExists, false)
	}

	// Regular environment variable lookup
	v, _ := syscall.Getenv(key)
	return v
}

// envHandleExpansion handles all expansion patterns
func envHandleExpansion(varName, value string, condition func(string, bool) bool, useVarValue bool) string {
	envValue, exists := syscall.Getenv(varName)
	if condition(envValue, exists) {
		if useVarValue {
			return envValue
		}
		return envExpandValue(value)
	}
	if useVarValue {
		return envExpandValue(value)
	}
	return ""
}

// envExpandValue expands variables and nested expressions in the value
func envExpandValue(value string) string {
	if strings.Contains(value, "$") {
		return os.Expand(value, Getenv)
	}
	return value
}
