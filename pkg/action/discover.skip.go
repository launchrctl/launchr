// Package action provides implementations of discovering and running actions.
package action

func skipSystemDirs(root string, path string) bool {
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
