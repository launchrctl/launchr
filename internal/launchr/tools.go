// Package launchr provides common app functionality.
package launchr

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
)

// GetFsAbsPath returns absolute path for a FS struct.
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
func GetTypePkgPathName(v interface{}) (string, string) {
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t.PkgPath(), t.Name()
}
