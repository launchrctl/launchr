package launchr

import (
	"fmt"
	"io/fs"
	"os"
	osuser "os/user"
	"path/filepath"
	"reflect"
	"strings"
)

// MustAbs returns absolute filepath and panics on error.
func MustAbs(path string) string {
	abs, err := filepath.Abs(filepath.Clean(filepath.FromSlash(path)))
	if err != nil {
		panic(err)
	}
	return abs
}

// MustSubFS returns an [FS] corresponding to the subtree rooted at fsys's dir.
func MustSubFS(fsys fs.FS, path string) fs.FS {
	sub, err := fs.Sub(fsys, path)
	if err != nil {
		panic(err)
	}
	return sub
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
	return isDotPath(path) || isHiddenPath(path)
}

func isDotPath(path string) bool {
	if path == "." {
		return false
	}
	dirs := strings.Split(filepath.ToSlash(path), "/")
	for _, v := range dirs {
		if v[0] == '.' {
			return true
		}
	}
	return false
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
	var err error
	u, err := osuser.Current()
	if err != nil {
		u = &osuser.User{}
	}
	baseCand := []string{
		// Linux tmpfs paths.
		"/dev/shm",           // Should be available for all.
		"/run/user/" + u.Uid, // User specific.
		"/run",               // Root.
		// Fallback to temp dir, it may not be written to disk if the files are small or deleted shortly.
		// It will be used for Windows and macOS.
		os.TempDir(),
	}
	basePath := ""
	dirPath := ""
	for _, cand := range baseCand {
		// Ensure base path exists
		var stat os.FileInfo
		if stat, err = os.Stat(cand); err == nil && stat.IsDir() {
			basePath = cand
			if name != "" {
				newBase := filepath.Join(basePath, name)
				err = os.Mkdir(newBase, 0700)
				if err != nil && !os.IsExist(err) {
					// Try next candidate.
					continue
				}
				basePath = newBase
			}

			// Create the directory
			dirPath, err = os.MkdirTemp(basePath, pattern)
			if err != nil {
				// Try next candidate.
				continue
			}
			// We found the candidate.
			break
		}
	}
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory '%s': %w", dirPath, err)
	}
	if dirPath == "" {
		return "", fmt.Errorf("failed to create temp directory")
	}
	return dirPath, nil
}

// MkdirTempWithCleanup creates a temporary directory with MkdirTemp.
// The temp directory is removed when the app terminates.
func MkdirTempWithCleanup(pattern string) (string, error) {
	dirPath, err := MkdirTemp(pattern)
	if err != nil {
		return "", err
	}

	// Make sure the dir is cleaned on finish.
	RegisterCleanupFn(func() error {
		return os.RemoveAll(dirPath)
	})

	return dirPath, nil
}

// EscapePathString escapes characters that may be
// incorrectly treated as a string like backshash "\" in a Windows path.
func EscapePathString(s string) string {
	if filepath.Separator == '/' {
		return s
	}
	return strings.Replace(s, "\\", "\\\\", -1)
}
