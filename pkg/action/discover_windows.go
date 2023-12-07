// Package action provides implementations of discovering and running actions.
package action

import (
	"path/filepath"
	"syscall"
)

func isHiddenFile(path string) bool {
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
