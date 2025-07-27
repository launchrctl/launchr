//go:build windows

package launchr

import (
	"golang.org/x/sys/windows"
)

const (
	allBytes = ^uint32(0)
)

func (f *LockedFile) lock(waitToAcquire bool) (err error) {
	lt := windows.LOCKFILE_EXCLUSIVE_LOCK
	if !waitToAcquire {
		lt = lt | windows.LOCKFILE_FAIL_IMMEDIATELY
	}
	ol := new(windows.Overlapped)
	err = windows.LockFileEx(windows.Handle(f.file.Fd()), uint32(lt), 0, allBytes, allBytes, ol)
	if err != nil {
		return err
	}
	return nil
}

func (f *LockedFile) unlock() {
	if !f.locked {
		// If we didn't lock the file, we shouldn't unlock it.
		return
	}
	ol := new(windows.Overlapped)
	err := windows.UnlockFileEx(windows.Handle(f.file.Fd()), 0, allBytes, allBytes, ol)
	if err != nil {
		Log().Warn("unlock is called on a not locked file", "err", err)
	}
	f.locked = false
}
