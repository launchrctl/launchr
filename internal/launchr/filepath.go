package launchr

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
)

// MustAbs returns absolute filepath and panics on error.
func MustAbs(path string) string {
	abs, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		panic(err)
	}
	return abs
}

// GetFsAbsPath returns absolute path for a [fs.FS] struct.
func GetFsAbsPath(fs fs.FS) string {
	cwd := ""
	rval := reflect.ValueOf(fs)
	if rval.Kind() == reflect.String {
		cwd = rval.String()
		// @todo Rethink absolute path usage overall.
		if !filepath.IsAbs(cwd) {
			cwd = MustAbs(cwd)
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

// IsHiddenPath checks if a path is hidden path.
func IsHiddenPath(path string) bool {
	return isHiddenPath(path)
}

// IsSystemPath checks if a path is a system path.
func IsSystemPath(root string, path string) bool {
	if root == "" {
		// We are in virtual FS.
		return false
	}

	dirs := []string{
		// Python specific.
		"__pycache__",
		"venv",
		// JS specific stuff.
		"node_modules",
		// Usually project dependencies.
		"vendor",
	}

	// Check application specific.
	if existsInSlice(dirs, path) {
		return true
	}
	// Skip in root.
	if isRootPath(root) && existsInSlice(skipRootDirs, path) {
		return true
	}
	// Skip user specific directories.
	if isUserHomeDir(path) && existsInSlice(skipUserDirs, path) {
		return true
	}

	return false
}

func existsInSlice[T comparable](slice []T, el T) bool {
	for _, v := range slice {
		if v == el {
			return true
		}
	}
	return false
}
