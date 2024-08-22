//go:build !windows
// +build !windows

package action

import (
	"path/filepath"
	"strings"
)

var skipRootDirs = []string{
	// Unix
	// It's not recommended to run in root,
	// but if so we will skip most of the paths.
	"bin",
	"sbin",
	"lib",
	"etc",
	"var",
	"tmp",
	"dev",
	"proc",
	"sys",
	"boot",
	"srv",

	// MacOs
	"System",
	"Library",
	"Applications",
}

var skipUserDirs = []string{
	// Go root is usually in home and have a lot of packages.
	"go",

	// MacOs
	"Applications",
	"Documents",
	"Desktop",
	"Downloads",
	"Library",
	"Music",
	"Pictures",
	"Movies",
	"Public",
}

func isHiddenPath(path string) bool {
	if path == "." {
		return false
	}
	dirs := strings.Split(path, string(filepath.Separator))
	for _, v := range dirs {
		if v[0] == '.' {
			return true
		}
	}

	return false
}

func isRootPath(path string) bool {
	return path == "/"
}

func isUserHomeDir(path string) bool {
	abs, _ := filepath.Abs(path)
	linux, _ := filepath.Match("/home/*/*", abs)
	macOs, _ := filepath.Match("/Users/*/*", abs)
	return linux || macOs
}
