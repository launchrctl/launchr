//go:build windows

package launchr

import (
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
