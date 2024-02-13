package action

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"golang.org/x/mod/sumdb/dirhash"

	"github.com/launchrctl/launchr/internal/launchr"
	"github.com/launchrctl/launchr/pkg/log"
)

const sumFilename = "actions.sum"

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
		log.Debug("unlock is called on a not locked file : %s", err)
	}
	f.locked = false
}

// ImageBuildCacheResolver is responsible for checking image build hash sums to rebuild images.
type ImageBuildCacheResolver struct {
	fname         string
	file          *lockedFile
	items         map[string]string
	requireUpdate bool
	cfg           launchr.Config
}

// NewImageBuildCacheResolver creates ImageBuildCacheResolver from global configuration.
func NewImageBuildCacheResolver(cfg launchr.Config) *ImageBuildCacheResolver {
	fname := cfg.Path(sumFilename)
	return &ImageBuildCacheResolver{
		cfg:   cfg,
		fname: fname,
		file:  &lockedFile{fname: fname},
		items: nil,
	}
}

// EnsureLoaded makes sure the sum file is loaded.
func (r *ImageBuildCacheResolver) EnsureLoaded() (err error) {
	if r.items == nil {
		r.items, err = r.readSums()
	}
	return err
}

func (r *ImageBuildCacheResolver) assertLoaded() {
	if r.items == nil {
		panic("actions.sum was not loaded, call Load first")
	}
}

func (r *ImageBuildCacheResolver) readSums() (map[string]string, error) {
	err := r.file.Open(os.O_RDONLY, 0)
	defer r.file.Close()
	if os.IsNotExist(err) {
		return make(map[string]string), nil
	} else if err != nil {
		return nil, err
	}

	items, err := parseSums(r.file.fname, r.file)
	if err != nil {
		return nil, err
	}

	return items, err
}

// DirHash calculates the hash of a directory specified by the path parameter.
func (r *ImageBuildCacheResolver) DirHash(path string) (string, error) {
	return dirhash.HashDir(path, "", dirhash.Hash1)
}

// GetSum returns a sum for an image tag.
func (r *ImageBuildCacheResolver) GetSum(tag string) string {
	r.assertLoaded()
	if tag == "" {
		panic("tag must not be empty")
	}
	if sum, ok := r.items[tag]; ok {
		return sum
	}

	return ""
}

// SetSum adds sum for a tag. Provide empty sum to remove it.
func (r *ImageBuildCacheResolver) SetSum(tag string, sum string) {
	r.assertLoaded()
	if tag == "" {
		panic("tag must not be empty")
	}

	r.items[tag] = sum
	r.requireUpdate = true
}

// Save saves the sum file to the persistent storage.
func (r *ImageBuildCacheResolver) Save() error {
	if !r.requireUpdate {
		return nil
	}
	r.assertLoaded()
	fileItems, err := r.readSums()
	if err != nil {
		return err
	}

	err = r.file.Open(os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0600)
	defer r.file.Close()
	if err != nil {
		return err
	}

	// merge new items with current file items
	merged := make(map[string]string)
	for k, v := range fileItems {
		merged[k] = v
	}

	for k, v := range r.items {
		merged[k] = v
		if v == "" {
			// Ensure deleted item won't be taken from old file values.
			delete(merged, k)
		}
	}
	r.items = merged

	// Save in alphabetical order.
	keys := make([]string, 0, len(r.items))
	for k := range r.items {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		_, err = fmt.Fprintf(r.file, "%s %s\n", k, r.items[k])
		if err != nil {
			return err
		}
	}

	return err
}

// Destroy removes the sum file from the persistent storage.
func (r *ImageBuildCacheResolver) Destroy() error {
	r.items = nil
	return r.file.Remove()
}

func parseSums(fname string, file io.Reader) (map[string]string, error) {
	items := make(map[string]string)
	scanner := bufio.NewScanner(file)
	lineno := 0
	for scanner.Scan() {
		lineno++
		f := strings.Fields(scanner.Text())
		if len(f) == 0 {
			continue
		}
		if len(f) > 2 {
			return nil, fmt.Errorf("malformed %s:\nline %d: wrong number of fields %v", fname, lineno, len(f))
		}

		items[f[0]] = f[1]
	}

	return items, nil
}
