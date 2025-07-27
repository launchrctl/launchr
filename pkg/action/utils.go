package action

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/launchrctl/launchr/internal/launchr"

	"gopkg.in/yaml.v3"
)

type userInfo struct {
	UID string
	GID string
}

func (u userInfo) String() string {
	return u.UID + ":" + u.GID
}

func yamlTypeError(s string) *yaml.TypeError {
	return &yaml.TypeError{Errors: []string{s}}
}

func yamlTypeErrorLine(s string, l int, c int) *yaml.TypeError {
	return yamlTypeError(fmt.Sprintf("%s, line %d, col %d", s, l, c))
}

func yamlMergeErrors(errs ...*yaml.TypeError) *yaml.TypeError {
	strs := make([]string, 0, len(errs))
	for _, err := range errs {
		if err != nil {
			strs = append(strs, err.Errors...)
		}
	}
	return &yaml.TypeError{Errors: strs}
}

func yamlFindNodeByKey(n *yaml.Node, k string) *yaml.Node {
	if n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i < len(n.Content); i += 2 {
		if n.Content[i].Value == k {
			return n.Content[i+1]
		}
	}
	return nil
}

func yamlNodeLineCol(n *yaml.Node, k string) (int, int) {
	if o := yamlFindNodeByKey(n, k); o != nil {
		return o.Line, o.Column
	}
	return n.Line, n.Column
}

// dupSet is a unique set of strings, checks is string is already added to the set.
type dupSet map[string]struct{}

var replDashes = strings.NewReplacer("-", "_")

func (d dupSet) isUnique(s string) bool {
	_, ok := d[s]
	_, okDashed := d[replDashes.Replace(s)]
	if ok || okDashed {
		return false
	}
	d[s] = struct{}{}
	return true
}

// yamlParseDefNodes contains the set of yaml nodes for parsing context.
// Used to identify unique names of action arguments and options.
type yamlParseDefNodes struct {
	nodes map[*yaml.Node]struct{}
	dups  dupSet
}

// yamlGlobalParseMeta has a yaml node tree defined per action [Definition].
// Used to have a context about during the parsing stage.
type yamlGlobalParseMeta struct {
	tree map[*Definition]yamlParseDefNodes
	mx   sync.RWMutex
}

func newGlobalYamlParseMeta() *yamlGlobalParseMeta {
	return &yamlGlobalParseMeta{
		tree: make(map[*Definition]yamlParseDefNodes),
	}
}

func (m *yamlGlobalParseMeta) addDef(d *Definition, n *yaml.Node) {
	m.mx.Lock()
	defer m.mx.Unlock()
	if _, ok := m.tree[d]; ok {
		return
	}
	nodes := collectAllNodes(n)
	mdef := yamlParseDefNodes{
		nodes: make(map[*yaml.Node]struct{}),
		dups:  make(dupSet),
	}
	for _, child := range nodes {
		mdef.nodes[child] = struct{}{}
	}
	m.tree[d] = mdef
}

func (m *yamlGlobalParseMeta) removeDef(d *Definition) {
	m.mx.Lock()
	defer m.mx.Unlock()
	delete(m.tree, d)
}

func (m *yamlGlobalParseMeta) dupsByNode(n *yaml.Node) dupSet {
	m.mx.RLock()
	defer m.mx.RUnlock()
	for _, md := range m.tree {
		if _, ok := md.nodes[n]; ok {
			return md.dups
		}
	}
	return nil
}

// collectAllNodes traverses all yaml tree and returns all nodes as a slice.
func collectAllNodes(n *yaml.Node) []*yaml.Node {
	res := make([]*yaml.Node, 0, len(n.Content)+1)
	res = append(res, n)
	for i := 0; i < len(n.Content); i++ {
		res = append(res, collectAllNodes(n.Content[i])...)
	}
	return res
}

// EnvVarRuntimeShellBash defines path to bash shell.
var EnvVarRuntimeShellBash = launchr.EnvVar("runtime_shell_bash")

func createRTShellBashContext(a *Action) (*shellContext, error) {
	path, err := getBashPath()
	if err != nil {
		return nil, err
	}
	return prepareShellContext(a, path)
}

func getBashPath() (string, error) {
	path, err := getRuntimeShellBashFromEnv()
	if err != nil {
		launchr.Log().Warn("failed to get shell from "+EnvVarRuntimeShellBash.String()+". Fallback to PATH lookup", "err", err)
	}
	if path != "" {
		return path, nil
	}
	path, err = exec.LookPath("bash")
	if err != nil {
		// Try to find bash.
		for _, path = range launchr.KnownBashPaths() {
			if _, err := os.Stat(path); err == nil {
				return path, nil
			}
		}
		return "", err
	}
	return path, nil
}

var errPathNotExecutable = fmt.Errorf("file is not executable")

func getRuntimeShellBashFromEnv() (string, error) {
	shell := EnvVarRuntimeShellBash.Get()
	if shell == "" {
		return "", nil
	}

	// Check if the file is executable.
	if err := isExecutable(shell); err != nil {
		return "", fmt.Errorf("runtime shell %q is not executable: %w", shell, err)
	}
	return shell, nil
}

// exportScriptToFile exports the shell script to a temporary file and returns the file path
func exportScriptToFile(script string) (string, error) {
	// Create temporary file with action-specific naming
	tmpDir, err := launchr.MkdirTempWithCleanup("runtime_shell_")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}
	path := filepath.Join(tmpDir, "action.sh")
	scriptFile, err := os.Create(path) //nolint:gosec // G304 We create the path.
	if err != nil {
		return "", fmt.Errorf("failed to create temp script file: %w", err)
	}
	defer scriptFile.Close()

	// Write the script content to the file
	if _, err := scriptFile.WriteString(script); err != nil {
		return "", fmt.Errorf("failed to write script to file: %w", err)
	}

	// Make the script executable
	if err := scriptFile.Chmod(0755); err != nil {
		return "", fmt.Errorf("failed to make script executable: %w", err)
	}

	return path, nil
}
