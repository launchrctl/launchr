package action

import (
	"io/fs"
	"math/rand"
	"path"
	"testing"
	"testing/fstest"

	"github.com/moby/moby/pkg/namesgenerator"
)

func Test_Discover(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name   string
		fs     fstest.MapFS
		expCnt int
	}

	allValid := _getFsMapActions(7, validEmptyVersionYaml, true)
	invalidYaml := _mergeFsMaps(
		_getFsMapActions(7, validEmptyVersionYaml, true),
		_getFsMapActions(3, invalidEmptyCmdYaml, true),
	)
	invalidPath := _mergeFsMaps(
		_getFsMapActions(7, validEmptyVersionYaml, true),
		_getFsMapActions(3, validEmptyVersionYaml, false),
	)

	// @todo test path contains 2 actions in same dir.
	tts := []testCase{
		// All yaml files are valid and discovered.
		{"all valid", allValid, 7},
		// Some yaml files are invalid and not taken in account.
		// @todo rethink how invalid yaml is discovered.
		{"invalid yaml", invalidYaml, 10},
		// Invalid yaml paths are ignored.
		{"invalid paths", invalidPath, 7},
	}
	for _, tt := range tts {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ad := NewYamlDiscovery(tt.fs)
			actions, err := ad.Discover()
			if err != nil {
				t.Errorf("unexpected error %v", err)
			}
			if tt.expCnt != len(actions) {
				t.Errorf("expected %d discovered actions, got %d", tt.expCnt, len(actions))
			}
		})
	}
}

type dirEntry string

func (d dirEntry) DirEntry() fs.DirEntry {
	tmpfs := fstest.MapFS{}
	ds := string(d)
	p := ds
	// If it's a dir path, add test file to return dir.
	if path.Ext(p) == "" {
		p = path.Join(p, "action.yaml")
	}
	tmpfs[p] = &fstest.MapFile{}
	f, _ := tmpfs.Open(ds)
	return f.(fs.DirEntry)
}

func Test_Discover_isValid(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name string
		path dirEntry
		exp  bool
	}

	tts := []testCase{
		{"valid yaml", "1/2/actions/3/action.yaml", true},                     // Valid action.yaml path.
		{"valid yml", "1/2/actions/3/action.yml", true},                       // Valid action.yml path.
		{"random file", "1/2/actions/3/random.yaml", false},                   // Random yaml name.
		{"incorrect filename prefix", "1/2/actions/3/myaction.yaml", false},   // Incorrect prefix.
		{"incorrect filename suffix", "1/2/actions/3/action.yaml.bkp", false}, // Incorrect suffix.
		{"incorrect path", "1/2/3/action.yaml", false},                        // File not inside an "actions" directory.
		{"incorrect hidden path", ".1/2/actions/3/action.yml", false},         // Invalid hidden directory
		{"nested action", "1/2/actions/3/4/5/action.yaml", false},             // There is a deeper nesting in actions directory.
		{"root action", "actions/verb/action.yaml", false},                    // Actions are located in root.
		{"dir", "1/2/actions/3", false},                                       // A directory is given.
	}

	// Run tests.
	ad := NewYamlDiscovery(fstest.MapFS{})
	for _, tt := range tts {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			res := ad.isValid(string(tt.path), tt.path.DirEntry())
			if tt.exp != res {
				t.Errorf("expected %t, got %t", tt.exp, res)
			}
		})
	}
}

func Test_Discover_getActionName(t *testing.T) {
	t.Parallel()
	type testCase struct {
		path string
		exp  string
	}
	tts := []testCase{
		// Expected relative path.
		{"path/to/my/actions/verb/action.yaml", "path.to.my:verb"},
		// Expected absolute path.
		{"/absolute/path/to/my/actions/verb/action.yaml", "absolute.path.to.my:verb"},
		// Missing /actions/ in the subpath.
		{"path/to/my/verb/action.yaml", ""},
		// Unexpected root path.
		{"actions/verb/action.yaml", ""},
		// Unexpected absolute root path.
		{"/actions/verb/action.yaml", ""},
		// Unexpected nested, but valid.
		{"1/2/3/actions/4/5/6/action.yaml", "1.2.3:4.5.6"},
		// Unexpected path, but valid.
		{"1/2/3/actions/4/5/6/random.yaml", "1.2.3:4.5.6"},
	}
	for _, tt := range tts {
		res := getActionID(tt.path)
		if tt.exp != res {
			t.Errorf("expected %q, got %q", tt.exp, res)
		}
	}
}

func _generateActionPath(d int, validPath bool) string {
	elems := make([]string, 0, d+3)
	for i := 0; i < d; i++ {
		elems = append(elems, namesgenerator.GetRandomName(0))
	}
	if validPath {
		elems = append(elems, actionsDirname)
	}
	elems = append(elems, namesgenerator.GetRandomName(0), "action.yaml")
	return path.Join(elems...)
}

func _getFsMapActions(num int, str string, validPath bool) fstest.MapFS {
	m := make(fstest.MapFS)
	for i := 0; i < num; i++ {
		fa := _generateActionPath(rand.Intn(5)+1, validPath) //nolint:gosec
		m[fa] = &fstest.MapFile{Data: []byte(str)}
	}
	return m
}

func _mergeFsMaps(maps ...fstest.MapFS) fstest.MapFS {
	m := make(fstest.MapFS)
	for _, mm := range maps {
		for k, v := range mm {
			m[k] = v
		}
	}
	return m
}
