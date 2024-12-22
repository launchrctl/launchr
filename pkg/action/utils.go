package action

import (
	"fmt"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

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

func copyMap[K comparable, V any](m map[K]V) map[K]V {
	r := make(map[K]V, len(m))
	for k, v := range m {
		r[k] = v
	}
	return r
}
