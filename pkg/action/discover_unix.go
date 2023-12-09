//go:build !windows
// +build !windows

package action

import (
	"path/filepath"
)

func isHidden(path string) bool {
	pathList := filepath.SplitList(path)
	for _, v := range pathList {
		if len(v) > 1 && v[0:1] == "." {
			return true
		}
	}

	return false
}
