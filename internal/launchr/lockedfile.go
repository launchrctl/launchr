package launchr

import (
	"os"
	"path/filepath"
)

// @todo refactor to use one implementation here and in keyring.

// LockedFile is file with a lock for other processes.
type LockedFile struct {
	fname  string
	file   *os.File
	locked bool
}

// NewLockedFile creates a new LockedFile.
func NewLockedFile(fname string) *LockedFile {
	return &LockedFile{fname: fname}
}

// Filename returns file's name.
func (f *LockedFile) Filename() string {
	return f.fname
}

// Open opens a file and locks it for other..
func (f *LockedFile) Open(flag int, perm os.FileMode) (err error) {
	isCreate := flag&os.O_CREATE == os.O_CREATE
	if isCreate {
		err = EnsurePath(filepath.Dir(f.fname))
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

// Read implements [io.ReadWriteCloser] interface.
func (f *LockedFile) Read(p []byte) (n int, err error) { return f.file.Read(p) }

// Write implements [io.ReadWriteCloser] interface.
func (f *LockedFile) Write(p []byte) (n int, err error) { return f.file.Write(p) }

// Close implements [io.ReadWriteCloser] interface.
func (f *LockedFile) Close() error {
	f.unlock()
	if f.file != nil {
		return f.file.Close()
	}
	return nil
}

// Remove deletes the file.
func (f *LockedFile) Remove() (err error) {
	err = os.Remove(f.fname)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
