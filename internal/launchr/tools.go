// Package launchr provides common app functionality.
package launchr

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"time"

	flag "github.com/spf13/pflag"
)

// GetFsAbsPath returns absolute path for a [fs.FS] struct.
func GetFsAbsPath(fs fs.FS) string {
	cwd := ""
	rval := reflect.ValueOf(fs)
	if rval.Kind() == reflect.String {
		var err error
		cwd = rval.String()
		// @todo Rethink absolute path usage overall.
		if !filepath.IsAbs(cwd) {
			cwd, err = filepath.Abs(cwd)
			if err != nil {
				panic("can't retrieve absolute path for the path")
			}
		}
	}
	return cwd
}

// EnsurePath creates all directories in the path.
func EnsurePath(parts ...string) error {
	p := filepath.Clean(filepath.Join(parts...))
	if _, err := os.Stat(p); os.IsNotExist(err) {
		return os.MkdirAll(p, 0750)
	}
	return nil
}

// GetTypePkgPathName returns type package path and name for internal usage.
func GetTypePkgPathName(v any) (string, string) {
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t.PkgPath(), t.Name()
}

// IsCommandErrHelp checks if an error is a flag help err used for intercommunication.
func IsCommandErrHelp(err error) bool {
	return errors.Is(err, flag.ErrHelp)
}

// EstimateTime returns a function that runs callback with
// the elapsed time between the call to timer and the call to
// the returned function. The returned function is intended to
// be used in a defer statement:
//
// defer EstimateTime("sum", func (diff time.Duration) { ... })().
func EstimateTime(fn func(diff time.Duration)) func() {
	start := time.Now()
	return func() {
		fn(time.Since(start))
	}
}

// IsSELinuxEnabled checks if selinux is enabled on the system.
func IsSELinuxEnabled() bool {
	// @todo it won't actually work with a remote environment.
	data, err := os.ReadFile("/sys/fs/selinux/enforce")
	if err != nil {
		return false
	}
	return string(data) == "1"
}
