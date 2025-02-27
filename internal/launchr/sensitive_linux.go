//go:build release

package launchr

import (
	"golang.org/x/sys/unix"
)

func init() {
	err := setProcNotDumpable()
	if err != nil {
		panic(err)
	}
}

func setProcNotDumpable() error {
	// Disable core dumps and prevent ptrace attacks
	err := unix.Prctl(unix.PR_SET_DUMPABLE, 0, 0, 0, 0)
	if err != nil {
		return err
	}
	return nil
}
