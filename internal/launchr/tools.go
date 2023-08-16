// Package launchr provides common app functionality.
package launchr

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

// ToCamelCase converts a string to CamelCase.
// @todo reference module and license.
func ToCamelCase(s string, capFirst bool) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}

	n := strings.Builder{}
	n.Grow(len(s))
	capNext := capFirst
	for i, v := range []byte(s) {
		vIsCap := v >= 'A' && v <= 'Z'
		vIsLow := v >= 'a' && v <= 'z'
		if capNext {
			if vIsLow {
				v += 'A'
				v -= 'a'
			}
		} else if i == 0 {
			if vIsCap {
				v += 'a'
				v -= 'A'
			}
		}
		if vIsCap || vIsLow {
			n.WriteByte(v)
			capNext = false
		} else if vIsNum := v >= '0' && v <= '9'; vIsNum {
			n.WriteByte(v)
			capNext = true
		} else {
			capNext = v == '_' || v == ' ' || v == '-' || v == '.'
		}
	}
	return n.String()
}
