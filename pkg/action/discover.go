// Package action provides implementations of discovering and running actions.
package action

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/launchrctl/launchr/internal/launchr"
	"github.com/launchrctl/launchr/pkg/log"
)

// Discovery finds action files and parses them.
type Discovery interface {
	Discover() ([]*Action, error)
}

type yamlDiscovery struct {
	fs        fs.FS
	cwd       string
	targetRgx *regexp.Regexp
}

var actionYamlRegex = regexp.MustCompile(`^action\.(yaml|yml)$`)

// NewYamlDiscovery creates an instance of action discovery.
func NewYamlDiscovery(fs fs.FS) Discovery {
	cwd := launchr.GetFsAbsPath(fs)
	return &yamlDiscovery{fs, cwd, actionYamlRegex}
}

const actionsDirname = "actions"

var actionsSubdir = strings.Join([]string{"", actionsDirname, ""}, string(filepath.Separator))

func (ad *yamlDiscovery) isValid(path string, d fs.DirEntry) bool {
	i := strings.LastIndex(path, actionsSubdir)
	return !d.IsDir() &&
		i != -1 &&
		strings.Count(path[i+len(actionsSubdir):], string(filepath.Separator)) == 1 && // Nested actions are not allowed.
		ad.targetRgx.MatchString(d.Name())
}

// findFiles searches for a filename in a given dir.
// Returns an array of relative file paths.
func (ad *yamlDiscovery) findFiles() chan string {
	ch := make(chan string, 10)
	go func() {
		err := fs.WalkDir(ad.fs, ".", func(path string, d fs.DirEntry, err error) error {
			if err == nil && ad.isValid(path, d) {
				ch <- path
			}

			return err
		})

		if err != nil {
			// @todo we shouldn't log here
			log.Err("%v", err)
		}

		close(ch)
	}()

	return ch
}

// @todo move somewhere
func timeTrack(start time.Time, name string) {
	log.Debug("%s took %s", name, time.Since(start))
}

// Discover traverses the file structure for a given discovery path dp.
// Returns array of ActionCommand.
// If a command is invalid, it's ignored.
func (ad *yamlDiscovery) Discover() ([]*Action, error) {
	defer timeTrack(time.Now(), "launchr.Discover")

	wg := sync.WaitGroup{}
	mx := sync.Mutex{}
	actions := make([]*Action, 0, 32)

	for f := range ad.findFiles() {
		wg.Add(1)
		go func(f string) {
			defer wg.Done()
			// @todo skip duplicate like action.yaml+action.yml, prefer yaml.
			a := ad.parseYamlAction(f)
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

// parseYamlAction parses yaml file f and returns available actions.
func (ad *yamlDiscovery) parseYamlAction(f string) *Action {
	id := getActionID(f)
	if id == "" {
		panic(fmt.Errorf("action id cannot be empty, file %q", f))
	}
	a := NewAction(id, ad.cwd, f)
	a.Loader = &yamlFileLoader{
		open: func() (fs.File, error) {
			return ad.fs.Open(f)
		},
		processor: NewPipeProcessor(
			escapeYamlTplCommentsProcessor{},
			envProcessor{},
			inputProcessor{},
		),
	}
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
