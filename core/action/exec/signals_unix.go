//go:build !windows
// +build !windows

package exec

import (
	"os"

	"golang.org/x/sys/unix"
)

func isRuntimeSig(s os.Signal) bool {
	return s == unix.SIGURG
}
