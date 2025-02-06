package action

import (
	"context"
	"io/fs"
	"path"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/launchrctl/launchr/internal/launchr"
)

func Test_Discover(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name   string
		fs     fstest.MapFS
		expCnt int
	}

	allValid := genFsTestMapActions(7, validEmptyVersionYaml, genPathTypeValid)
	invalidYaml := mergeFsTestMaps(
		genFsTestMapActions(7, validEmptyVersionYaml, genPathTypeValid),
		genFsTestMapActions(3, invalidEmptyCmdYaml, genPathTypeValid),
	)
	invalidPath := mergeFsTestMaps(
		genFsTestMapActions(7, validEmptyVersionYaml, genPathTypeValid),
		genFsTestMapActions(3, validEmptyVersionYaml, genPathTypeArbitrary),
		genFsTestMapActions(3, validEmptyVersionYaml, genPathTypeGHActions),
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
	ctx := context.Background()
	for _, tt := range tts {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ad := NewYamlDiscovery(NewDiscoveryFS(tt.fs, ""))
			actions, err := ad.Discover(ctx)
			if err != nil {
				t.Errorf("unexpected error %v", err)
			}
			if tt.expCnt != len(actions) {
				t.Errorf("expected %d discovered actions, got %d", tt.expCnt, len(actions))
			}
		})
	}
}

func Test_Discover_ActionWD(t *testing.T) {
	// Test if working directory is correctly set to actions on discovery.
	tfs := genFsTestMapActions(1, validEmptyVersionYaml, genPathTypeValid)
	var expFPath string
	for expFPath = range tfs {
		break
	}
	expectedWD := "expectedWD"
	ad := NewYamlDiscovery(NewDiscoveryFS(tfs, expectedWD))
	ctx := context.Background()
	actions, err := ad.Discover(ctx)
	require.NoError(t, err)
	assert.Equal(t, expFPath, actions[0].fpath)
	assert.Equal(t, launchr.MustAbs(expectedWD), actions[0].wd)

	ad = NewYamlDiscovery(NewDiscoveryFS(tfs, ""))
	actions, err = ad.Discover(ctx)
	require.NoError(t, err)
	assert.Equal(t, expFPath, actions[0].fpath)
	assert.Equal(t, launchr.MustAbs(""), actions[0].wd)
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
		{"valid yaml", "1/2/actions/3/action.yaml", true},                           // Valid action.yaml path.
		{"valid yml", "1/2/actions/3/action.yml", true},                             // Valid action.yml path.
		{"random file", "1/2/actions/3/random.yaml", false},                         // Random yaml name.
		{"incorrect filename prefix", "1/2/actions/3/myaction.yaml", false},         // Incorrect prefix.
		{"incorrect filename suffix", "1/2/actions/3/action.yaml.bkp", false},       // Incorrect suffix.
		{"incorrect path", "1/2/3/action.yaml", false},                              // File not inside an "actions" directory.
		{"incorrect hidden root path", ".1/2/actions/3/action.yml", false},          // Invalid hidden directory.
		{"incorrect hidden subdir path", "1/2/.github/actions/3/action.yml", false}, // Invalid hidden subdirectory.
		{"nested action", "1/2/actions/3/4/5/action.yaml", false},                   // There is a deeper nesting in actions directory.
		{"root action", "actions/verb/action.yaml", false},                          // Actions are located in root.
		{"dir", "1/2/actions/3", false},                                             // A directory is given.
	}

	// Run tests.
	ad := NewYamlDiscovery(NewDiscoveryFS(fstest.MapFS{}, ""))
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

func Test_Discover_IDProvider(t *testing.T) {
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
	idp := DefaultIDProvider{}
	for _, tt := range tts {
		res := idp.GetID(&Action{fpath: tt.path})
		if tt.exp != res {
			t.Errorf("expected %q, got %q", tt.exp, res)
		}
	}
}
