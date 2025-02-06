package launchr

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
)

// MustAbs returns absolute filepath and panics on error.
func MustAbs(path string) string {
	abs, err := filepath.Abs(filepath.Clean(filepath.FromSlash(path)))
	if err != nil {
		panic(err)
	}
	return abs
}

// FsRealpath returns absolute path for a [fs.FS] interface.
func FsRealpath(fsys fs.FS) string {
	if fsys == nil {
		return ""
	}
	fspath := ""
	rval := reflect.ValueOf(fsys)
	if rval.Kind() == reflect.String {
		fspath = rval.String()
		if !filepath.IsAbs(fspath) {
			return MustAbs(fspath)
		}
	}
	if typeString(fsys) == "*fs.subFS" {
		pfs := privateFieldValue[fs.FS](fsys, "fsys")
		dir := privateFieldValue[string](fsys, "dir")
		path := FsRealpath(pfs)
		if path != "" {
			return filepath.Join(path, dir)
		}
	}
	return fspath
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

// MkdirTemp creates a temporary directory.
// It tries to create a directory in memory (tmpfs).
func MkdirTemp(pattern string) (string, error) {
	baseCand := []string{
		// Linux tmpfs paths.
		"/run",
		// Fallback to temp dir, it may not be written to disk if the files are small or deleted shortly.
		os.TempDir(),
	}
	basePath := ""
	for _, cand := range baseCand {
		// Ensure base path exists
		if stat, err := os.Stat(cand); err == nil && stat.IsDir() {
			basePath = cand
			break
		}
	}
	if basePath == "" {
		return "", fmt.Errorf("no access temp directory")
	}
	if name != "" {
		newBase := filepath.Join(basePath, name)
		err := os.Mkdir(newBase, 0750)
		if err != nil {
			basePath = newBase
		}
	}

	// Create the directory
	dirPath, err := os.MkdirTemp(basePath, pattern)
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory '%s': %w", dirPath, err)
	}
	// Make sure the dir is cleaned on finish.
	RegisterCleanupFn(func() error {
		return os.RemoveAll(dirPath)
	})

	return dirPath, nil
}
