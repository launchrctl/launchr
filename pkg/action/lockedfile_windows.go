//go:build windows

package action

import (
	"golang.org/x/sys/windows"

	"github.com/launchrctl/launchr/pkg/log"
)

const (
	allBytes = ^uint32(0)
)

func (f *lockedFile) lock(waitToAcquire bool) (err error) {
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

func (f *lockedFile) unlock() {
	if !f.locked {
		// If we didn't lock the file, we shouldn't unlock it.
		return
	}
	ol := new(windows.Overlapped)
	err := windows.UnlockFileEx(windows.Handle(f.file.Fd()), 0, allBytes, allBytes, ol)
	if err != nil {
		log.Debug("unlock is called on a not locked file: %s", err)
	}
	f.locked = false
}
