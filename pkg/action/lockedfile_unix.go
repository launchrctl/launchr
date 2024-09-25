//go:build unix

package action

import (
	"syscall"

	"github.com/launchrctl/launchr/internal/launchr"
)

func (f *lockedFile) lock(waitToAcquire bool) (err error) {
	if f.locked {
		// If you get this error, there is racing between goroutines.
		panic("can't lock already opened file")
	}
	lockType := syscall.LOCK_EX
	if !waitToAcquire {
		lockType = lockType | syscall.LOCK_NB
	}
	err = syscall.Flock(int(f.file.Fd()), lockType)
	if err != nil {
		return err
	}
	f.locked = true

	return nil
}

func (f *lockedFile) unlock() {
	if !f.locked {
		// If we didn't lock the file, we shouldn't unlock it.
		return
	}
	if err := syscall.Flock(int(f.file.Fd()), syscall.LOCK_UN); err != nil {
		launchr.Log().Warn("unlock is called on a not locked file", "error", err)
	}
	f.locked = false
}
