//go:build unix

package launchr

import (
	"syscall"
)

func (f *LockedFile) lock(waitToAcquire bool) (err error) {
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

func (f *LockedFile) unlock() {
	if !f.locked {
		// If we didn't lock the file, we shouldn't unlock it.
		return
	}
	if err := syscall.Flock(int(f.file.Fd()), syscall.LOCK_UN); err != nil {
		Log().Warn("unlock is called on a not locked file", "error", err)
	}
	f.locked = false
}
