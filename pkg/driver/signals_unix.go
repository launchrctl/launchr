//go:build unix

package driver

import (
	"os"

	"golang.org/x/sys/unix"
)

func isRuntimeSig(s os.Signal) bool {
	return s == unix.SIGURG
}
