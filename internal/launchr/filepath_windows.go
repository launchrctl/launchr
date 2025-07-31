//go:build windows

package launchr

import (
	"os"
	"path/filepath"
	"syscall"
)

var skipRootDirs = []string{
	"Windows",
	"Program Files",
	"Program Files (x86)",
	"ProgramData",
}

var skipUserDirs = []string{
	"AppData",
	"Desktop",
	"Documents",
	"Downloads",
	"Music",
	"Pictures",
	"Videos",
	"Favorites",
	"Public",
}

func isHiddenPath(path string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	pointer, err := syscall.UTF16PtrFromString(`\\?\` + absPath)
	if err != nil {
		return false
	}

	attributes, err := syscall.GetFileAttributes(pointer)
	if err != nil {
		return false
	}

	return attributes&syscall.FILE_ATTRIBUTE_HIDDEN != 0
}

func isRootPath(path string) bool {
	return path == `C:\`
}

func isUserHomeDir(path string) bool {
	abs, _ := filepath.Abs(path)
	win, _ := filepath.Match(`C:\Users\*\*`, abs)
	return win
}

// KnownBashPaths returns paths where bash can be found. Used when PATH is not available.
func KnownBashPaths() []string {
	// System-wide installation paths
	paths := []string{
		"C:\\msys64\\usr\\bin\\bash.exe",
		"C:\\msys32\\usr\\bin\\bash.exe",
		"C:\\Program Files\\Git\\bin\\bash.exe",
		"C:\\Program Files (x86)\\Git\\bin\\bash.exe",
		"C:\\cygwin64\\bin\\bash.exe",
		"C:\\cygwin\\bin\\bash.exe",
	}

	// Get user's home directory
	userHome, err := os.UserHomeDir()
	if err == nil {
		// User-specific installation paths
		paths = append([]string{
			filepath.Join(userHome, "scoop", "apps", "git", "current", "bin", "bash.exe"),
			filepath.Join(userHome, "AppData", "Local", "Programs", "Git", "bin", "bash.exe"),
			filepath.Join(userHome, ".gitbash", "bin", "bash.exe"),
		}, paths...)
	}
	return paths
}
