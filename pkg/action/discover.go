// Package action provides implementations of discovering and running actions.
package action

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/launchrctl/launchr/internal/launchr"
	"github.com/launchrctl/launchr/pkg/log"
)

const actionsDirname = "actions"

var actionsSubdir = strings.Join([]string{"", actionsDirname, ""}, string(filepath.Separator))

// DiscoveryPlugin is a launchr plugin to discover actions.
type DiscoveryPlugin interface {
	launchr.Plugin
	DiscoverActions(fs fs.FS) ([]*Action, error)
}

// FileLoadFn is a type for loading a file.
type FileLoadFn func() (fs.File, error)

// DiscoveryStrategy is a way files will be discovered and loaded.
type DiscoveryStrategy interface {
	IsValid(name string) bool
	Loader(l FileLoadFn, p ...LoadProcessor) Loader
}

// Discovery defines a common functionality for discovering action files.
type Discovery struct {
	fs  fs.FS
	cwd string
	s   DiscoveryStrategy
}

// NewDiscovery creates an instance of action discovery.
func NewDiscovery(fs fs.FS, ds DiscoveryStrategy) *Discovery {
	cwd := launchr.GetFsAbsPath(fs)
	return &Discovery{fs, cwd, ds}
}

func (ad *Discovery) isValid(path string, d fs.DirEntry) bool {
	i := strings.LastIndex(path, actionsSubdir)

	if d.IsDir() || i == -1 || isHidden(path) {
		return false
	}

	return strings.Count(path[i+len(actionsSubdir):], string(filepath.Separator)) == 1 && // Nested actions are not allowed.
		ad.s.IsValid(d.Name())
}

// findFiles searches for a filename in a given dir.
// Returns an array of relative file paths.
func (ad *Discovery) findFiles() chan string {
	ch := make(chan string, 10)
	go func() {
		err := fs.WalkDir(ad.fs, ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if d.IsDir() && isHidden(path) {
				return fs.SkipDir
			}

			if ad.isValid(path, d) {
				ch <- path
			}

			return nil
		})

		if err != nil {
			// @todo we shouldn't log here
			log.Err("%v", err)
		}

		close(ch)
	}()

	return ch
}

// Discover traverses the file structure for a given discovery path.
// Returns array of Action.
// If an action is invalid, it's ignored.
func (ad *Discovery) Discover() ([]*Action, error) {
	wg := sync.WaitGroup{}
	mx := sync.Mutex{}
	actions := make([]*Action, 0, 32)

	for f := range ad.findFiles() {
		wg.Add(1)
		go func(f string) {
			defer wg.Done()
			// @todo skip duplicate like action.yaml+action.yml, prefer yaml.
			a := ad.parseFile(f)
			mx.Lock()
			defer mx.Unlock()
			actions = append(actions, a)
		}(f)
	}

	wg.Wait()

	// Sort alphabetically.
	sort.Slice(actions, func(i, j int) bool {
		return actions[i].ID < actions[j].ID
	})
	return actions, nil
}

// parseFile parses file f and returns an action.
func (ad *Discovery) parseFile(f string) *Action {
	id := getActionID(f)
	if id == "" {
		panic(fmt.Errorf("action id cannot be empty, file %q", f))
	}
	a := NewAction(id, ad.cwd, f)
	a.Loader = ad.s.Loader(
		func() (fs.File, error) { return ad.fs.Open(f) },
		envProcessor{},
		inputProcessor{},
	)
	return a
}

// getActionID parses filename and returns CLI command name.
// Empty string if the command name can't be generated.
func getActionID(f string) string {
	s := filepath.Dir(f)
	i := strings.LastIndex(s, actionsSubdir)
	if i == -1 {
		return ""
	}
	s = s[:i] + strings.Replace(s[i:], actionsSubdir, ":", 1)
	s = strings.ReplaceAll(s, string(filepath.Separator), ".")
	if s[0] == ':' {
		// Root paths are not allowed.
		return ""
	}
	s = strings.Trim(s, ".:")
	return s
}
