package launchr

import (
	"github.com/launchrctl/launchr/internal/launchr"
)

// Reexport for usage by other modules.
var (
	// GetFsAbsPath returns absolute path for an FS struct.
	GetFsAbsPath = launchr.GetFsAbsPath
	// EnsurePath creates all directories in the path.
	EnsurePath = launchr.EnsurePath
)
