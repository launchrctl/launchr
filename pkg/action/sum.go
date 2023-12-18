package action

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"github.com/launchrctl/launchr/internal/launchr"
	"github.com/launchrctl/launchr/pkg/log"
)

const sumFilename = "actions.sum"
const emptyItemSum = "h1:0"
const hashPrefix = "launchr"

var (
	ErrEmptySumFields = errors.New("sum item can't be empty") // ErrEmptySumFields if fields are empty
)

type localFile struct {
	fname string
	file  *os.File
}

func (f *localFile) Open(flag int, perm os.FileMode) (size int64, err error) {
	isCreate := flag&os.O_CREATE == os.O_CREATE
	if isCreate {
		err = launchr.EnsurePath(filepath.Dir(f.fname))
		if err != nil {
			return 0, err
		}
	}
	file, err := os.OpenFile(f.fname, flag, perm) //nolint:gosec
	if err != nil {
		return 0, err
	}

	f.file = file
	err = f.Lock(true)
	if err != nil {
		return 0, err
	}

	stat, err := file.Stat()
	if err != nil {
		return 0, err
	}

	return stat.Size(), err
}

func (f *localFile) Read(p []byte) (n int, err error)  { return f.file.Read(p) }
func (f *localFile) Write(p []byte) (n int, err error) { return f.file.Write(p) }
func (f *localFile) Close() error {
	return f.file.Close()
}
func (f *localFile) Remove() (err error) {
	err = os.Remove(f.fname)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
func (f *localFile) Lock(waitToAcquire bool) (err error) {
	lockType := syscall.LOCK_EX
	if !waitToAcquire {
		lockType = lockType | syscall.LOCK_NB
	}

	return syscall.Flock(int(f.file.Fd()), lockType)
}

func (f *localFile) Unlock() {
	if err := syscall.Flock(int(f.file.Fd()), syscall.LOCK_UN); err != nil {
		log.Warn("Unlock is being called on not locked file: %s", err)
	}
}

// SumItem stores action ID and dir checksum.
type SumItem struct {
	id  string
	sum string
}

type imageBuildCacheResolver struct {
	fname         string
	file          *localFile
	items         map[string]string
	loaded        bool
	requireUpdate bool
	cfg           launchr.Config
	dry           bool
}

func newImageBuildCacheResolver(cfg launchr.Config, dry bool) imageBuildCacheResolver {
	fname := cfg.Path(sumFilename)
	return imageBuildCacheResolver{
		cfg:   cfg,
		fname: fname,
		file:  &localFile{fname: fname},
		items: make(map[string]string),
		dry:   dry,
	}
}

// IsDryRun returns if cache resolver runs in dry mode.
func (r *imageBuildCacheResolver) IsDryRun() bool {
	return r.dry
}

func (r *imageBuildCacheResolver) load() error {
	if r.loaded || r.IsDryRun() {
		return nil
	}

	items, err := r.getItems()
	if err != nil {
		return err
	}

	r.items = items
	r.loaded = true
	return nil
}

func (r *imageBuildCacheResolver) getItems() (map[string]string, error) {
	defer r.file.Unlock()

	fsize, err := r.file.Open(os.O_RDONLY, 0)
	items := make(map[string]string)

	if os.IsNotExist(err) {
		r.loaded = true
		return items, nil
	} else if err != nil {
		return nil, err
	}

	data := make([]byte, fsize)
	_, err = r.file.Read(data)
	if err != nil {
		return items, err
	}

	items, err = parseSums(r.file.fname, data)
	if err != nil {
		return nil, err
	}

	return items, err
}

// GetItem returns a sum item by a action ID.
func (r *imageBuildCacheResolver) GetItem(image string) (SumItem, error) {
	if err := r.load(); err != nil {
		return SumItem{}, err
	}

	if k, ok := r.items[image]; ok {
		return SumItem{id: k, sum: r.items[image]}, nil
	}

	return SumItem{}, nil
}

// AddItem adds a new sum item.
func (r *imageBuildCacheResolver) AddItem(item SumItem) error {
	if item.id == "" || item.sum == "" {
		return ErrEmptySumFields
	}

	if err := r.load(); err != nil {
		return err
	}

	if sum, ok := r.items[item.id]; ok {
		if sum != item.sum {
			r.items[item.id] = item.sum
			r.requireUpdate = true
		}
	} else {
		r.items[item.id] = item.sum
		r.requireUpdate = true
	}

	return nil
}

// RemoveItem deletes an item by action ID.
func (r *imageBuildCacheResolver) RemoveItem(id string) error {
	if err := r.load(); err != nil {
		return err
	}

	if _, ok := r.items[id]; ok {
		r.items[id] = emptyItemSum
		r.requireUpdate = true
	}

	return nil
}

// // Save saves the sum file to the persistent storage.
func (r *imageBuildCacheResolver) Save() error {
	if !r.requireUpdate || r.IsDryRun() {
		return nil
	}

	fitems, err := r.getItems()
	if err != nil {
		return err
	}

	defer r.file.Unlock()
	fsize, err := r.file.Open(os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}

	defer r.file.Close()

	// merge new items with current file items
	merged := make(map[string]string)
	for k, v := range fitems {
		merged[k] = v
	}

	for k, v := range r.items {
		merged[k] = v
		if v == emptyItemSum {
			// Ensure deleted item won't be taken from old file values.
			delete(merged, k)
		}
	}

	data := make([]byte, fsize)
	_, err = r.file.Read(data)
	if err != nil {
		return err
	}

	var b bytes.Buffer
	keys := make([]string, 0, len(merged))
	for k := range merged {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		fmt.Fprintf(&b, "%s %s\n", k, merged[k])
	}

	_, err = r.file.Write(b.Bytes())

	return err
}

// Destroy removes the sum file from the persistent storage.
func (r *imageBuildCacheResolver) Destroy() error {
	return r.file.Remove()
}

func parseSums(fname string, data []byte) (map[string]string, error) {
	items := make(map[string]string)
	lineno := 0
	for len(data) > 0 {
		var line []byte
		lineno++
		i := bytes.IndexByte(data, '\n')
		if i < 0 {
			line, data = data, nil
		} else {
			line, data = data[:i], data[i+1:]
		}

		f := strings.Fields(string(line))
		if len(f) == 0 {
			continue
		}

		if len(f) != 2 {
			return nil, fmt.Errorf("malformed actions.sum:\n%s:%d: wrong number of fields %v", fname, lineno, len(f))
		}

		items[f[0]] = f[1]
	}

	return items, nil
}
