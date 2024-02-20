package action

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"golang.org/x/mod/sumdb/dirhash"

	"github.com/launchrctl/launchr/internal/launchr"
)

const sumFilename = "actions.sum"

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
