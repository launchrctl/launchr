// Package action provides implementations of discovering and running actions.
package action

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/launchrctl/launchr/internal/launchr"
	"github.com/launchrctl/launchr/pkg/log"
)

const actionsDirname = "actions"

var actionsSubdir = strings.Join([]string{"", actionsDirname, ""}, string(filepath.Separator))

// DiscoveryPlugin is a launchr plugin to discover actions.
type DiscoveryPlugin interface {
	launchr.Plugin
	DiscoverActions(ctx context.Context, fs launchr.ManagedFS, idp IDProvider) ([]*Action, error)
}

// AlterActionsPlugin is a launchr plugin to alter registered actions.
type AlterActionsPlugin interface {
	launchr.Plugin
	AlterActions() error
}

// DiscoveryFS is a file system to discover actions.
type DiscoveryFS struct {
	fs fs.FS
	wd string
}

// NewDiscoveryFS creates a DiscoveryFS given fs - a filesystem to discover
// and wd - working directory for an action, leave empty for current path.
func NewDiscoveryFS(fs fs.FS, wd string) DiscoveryFS { return DiscoveryFS{fs, wd} }

// FS implements launchr.ManagedFS.
func (f DiscoveryFS) FS() fs.FS { return f.fs }

// Open implements fs.FS and decorates the managed fs.
func (f DiscoveryFS) Open(name string) (fs.File, error) {
	return f.FS().Open(name)
}

// FileLoadFn is a type for loading a file.
type FileLoadFn func() (fs.File, error)

// DiscoveryStrategy is a way files will be discovered and loaded.
type DiscoveryStrategy interface {
	IsValid(name string) bool
	Loader(l FileLoadFn, p ...LoadProcessor) Loader
}

// IDProvider provides an ID for an action.
// It is used to generate an ID from an action declaration.
// DefaultIDProvider is the default implementation based on action filepath.
type IDProvider interface {
	GetID(a *Action) string
}

// Discovery defines a common functionality for discovering action files.
type Discovery struct {
	fs    DiscoveryFS
	fsDir string
	ds    DiscoveryStrategy
	idp   IDProvider
}

// NewDiscovery creates an instance of action discovery.
func NewDiscovery(fs DiscoveryFS, ds DiscoveryStrategy) *Discovery {
	fsDir := launchr.GetFsAbsPath(fs.fs)
	return &Discovery{
		fs:    fs,
		fsDir: fsDir,
		ds:    ds,
		idp:   DefaultIDProvider{},
	}
}

func (ad *Discovery) isValid(path string, d fs.DirEntry) bool {
	i := strings.LastIndex(path, actionsSubdir)

	// Invalid paths for action definition file.
	if d.IsDir() ||
		// No "actions" directory in the path.
		i == -1 ||
		// Must not be hidden itself.
		isHiddenPath(path) ||
		// Count depth of directories inside actions, must be only 1, not deeper.
		// Nested actions are not allowed.
		// dir/actions/1/action.yaml - OK, dir/actions/1/2/action.yaml - NOK.
		strings.Count(path[i+len(actionsSubdir):], string(filepath.Separator)) > 1 {
		return false
	}

	return ad.ds.IsValid(d.Name())
}

// findFiles searches for a filename in a given dir.
// Returns an array of relative file paths.
func (ad *Discovery) findFiles(ctx context.Context) chan string {
	ch := make(chan string, 10)
	go func() {
		longOpTimeout := time.After(5 * time.Second)
		err := fs.WalkDir(ad.fs, ".", func(path string, d fs.DirEntry, err error) error {
			select {
			// Show feedback on a long-running walk.
			case <-longOpTimeout:
				log.Warn("It takes more time than expected to discover actions.\nProbably you are running outside a project directory.")
			// Stop walking if the context has expired.
			case <-ctx.Done():
				return fs.SkipAll
			default:
				// Continue to scan.
			}
			// Skip OS specific directories to prevent going too deep.
			// Skip hidden directories.
			if d != nil && d.IsDir() && (isHiddenPath(path) || skipSystemDirs(ad.fsDir, path)) {
				return fs.SkipDir
			}
			if err != nil {
				// Skip dir on access denied.
				if os.IsPermission(err) && d.IsDir() {
					return fs.SkipDir
				}
				// Stop walking on unknown error.
				return err
			}

			// Check if the file is a candidate to be an action file.
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
func (ad *Discovery) Discover(ctx context.Context) ([]*Action, error) {
	defer log.DebugTimer("Action discovering")()
	wg := sync.WaitGroup{}
	mx := sync.Mutex{}
	actions := make([]*Action, 0, 32)

	// Traverse the FS.
	for f := range ad.findFiles(ctx) {
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
	a := NewAction(absPath(ad.fs.wd), ad.fsDir, f)
	a.Loader = ad.ds.Loader(
		func() (fs.File, error) { return ad.fs.Open(f) },
		envProcessor{},
		inputProcessor{},
	)
	// Assign ID to an action.
	a.ID = ad.idp.GetID(a)
	if a.ID == "" {
		panic(fmt.Errorf("action id cannot be empty, file %q", f))
	}

	return a
}

// SetActionIDProvider sets discovery specific action id provider.
func (ad *Discovery) SetActionIDProvider(idp IDProvider) {
	ad.idp = idp
}

// DefaultIDProvider is a default action id provider.
// It generates action id by a filepath.
type DefaultIDProvider struct{}

// GetID implements IDProvider interface.
// It parses action filename and returns CLI command name.
// Empty string if the command name can't be generated.
func (idp DefaultIDProvider) GetID(a *Action) string {
	f := a.fpath
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
