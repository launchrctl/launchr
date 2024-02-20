package action

import (
	"os"
	"path/filepath"

	"github.com/launchrctl/launchr/internal/launchr"
)

// @todo refactor to use one implementation here and in keyring.
type lockedFile struct {
	fname  string
	file   *os.File
	locked bool
}

func (f *lockedFile) Open(flag int, perm os.FileMode) (err error) {
	isCreate := flag&os.O_CREATE == os.O_CREATE
	if isCreate {
		err = launchr.EnsurePath(filepath.Dir(f.fname))
		if err != nil {
			return err
		}
	}
	f.file, err = os.OpenFile(f.fname, flag, perm) //nolint:gosec
	if err != nil {
		return err
	}

	err = f.lock(true)
	if err != nil {
		return err
	}

	return nil
}

func (f *lockedFile) Read(p []byte) (n int, err error)  { return f.file.Read(p) }
func (f *lockedFile) Write(p []byte) (n int, err error) { return f.file.Write(p) }

func (f *lockedFile) Close() error {
	f.unlock()
	if f.file != nil {
		return f.file.Close()
	}
	return nil
}

func (f *lockedFile) Remove() (err error) {
	err = os.Remove(f.fname)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
