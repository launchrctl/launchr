// Package action provides implementations of discovering and running actions.
package action

import (
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/launchrctl/launchr/pkg/cli"
	"github.com/launchrctl/launchr/pkg/log"
)

// Discovery finds action files and parses them.
type Discovery interface {
	Discover() ([]*Command, error)
}

type yamlDiscovery struct {
	fs        fs.FS
	cwd       string
	targetRgx *regexp.Regexp
}

var actionYamlRegex = regexp.MustCompile(`^action\.(yaml|yml)$`)

// NewYamlDiscovery creates an instance of action discovery.
func NewYamlDiscovery(fs fs.FS) Discovery {
	cwd := cli.GetFsAbsPath(fs)
	return &yamlDiscovery{fs, cwd, actionYamlRegex}
}

func (ad *yamlDiscovery) isValid(path string, d fs.DirEntry) bool {
	sub := "/actions/"
	i := strings.LastIndex(path, sub)
	return !d.IsDir() &&
		i != -1 &&
		strings.Count(path[i+len(sub):], "/") == 1 && // Nested actions are not allowed.
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
func (ad *yamlDiscovery) Discover() ([]*Command, error) {
	defer timeTrack(time.Now(), "launchr.Discover")

	wg := sync.WaitGroup{}
	mx := sync.Mutex{}
	cmds := make([]*Command, 0, 32)

	for f := range ad.findFiles() {
		wg.Add(1)
		go func(f string) {
			defer wg.Done()
			// @todo skip duplicate like action.yaml+action.yml, prefer yaml.
			if cmd, err := ad.parseYamlAction(f); err == nil {
				mx.Lock()
				cmds = append(cmds, cmd)
				mx.Unlock()
			} else {
				log.Warn("%v", err)
			}
		}(f)
	}

	wg.Wait()

	// Sort alphabetically.
	sort.Slice(cmds, func(i, j int) bool {
		return cmds[i].CommandName < cmds[j].CommandName
	})
	return cmds, nil
}

// parseYamlAction parses yaml file f and returns available commands.
func (ad *yamlDiscovery) parseYamlAction(f string) (*Command, error) {
	cmdName := getCmdMachineName(f)
	if cmdName == "" {
		return nil, fmt.Errorf("command name cannot be empty, file %s", f)
	}
	cmd := &Command{
		WorkingDir:  ad.cwd,
		Filepath:    f,
		CommandName: cmdName,
	}
	cmd.Loader = &yamlFileLoader{
		open: func() (fs.File, error) {
			return ad.fs.Open(f)
		},
		processor: NewPipeProcessor(
			&envProcessor{},
			&inputProcessor{cmd: cmd},
		),
	}
	return cmd, nil
}

// getCmdMachineName parses filename and returns CLI command name.
// Empty string if the command name can't be generated.
func getCmdMachineName(f string) string {
	// @todo does it work on Win?
	cmd := path.Dir(f)
	i := strings.LastIndex(cmd, "/actions/")
	if i == -1 {
		return ""
	}
	cmd = cmd[:i] + strings.Replace(cmd[i:], "/actions/", ":", 1)
	cmd = strings.ReplaceAll(cmd, "/", ".")
	if cmd[0] == ':' {
		// Root paths are not allowed.
		return ""
	}
	cmd = strings.Trim(cmd, ".:")
	return cmd
}
