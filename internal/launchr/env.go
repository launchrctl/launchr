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
	// Replace all subexpressions.
	if strings.Contains(key, "$") {
		key = os.Expand(key, Getenv)
	}
	// @todo implement ${var-$DEFAULT}, ${var:-$DEFAULT}, ${var+$DEFAULT}, ${var:+$DEFAULT},
	v, _ := syscall.Getenv(key)
	return v
}
