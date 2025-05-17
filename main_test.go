package launchr

import (
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
)

func TestMain(m *testing.M) {
	testscript.Main(m, map[string]func(){
		"launchr": RunAndExit,
	})
}

// TODO: Implement test groups build/runtime/unit
// TestScriptBuild tests how binary builds and outputs version.
func TestScriptBuild(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	testscript.Run(t, testscript.Params{
		Dir:                 "test/testdata/build",
		RequireExplicitExec: true,
		RequireUniqueNames:  true,
		Setup: func(env *testscript.Env) error {
			repoPath := MustAbs("./")
			env.Vars = append(
				env.Vars,
				"REPO_PATH="+repoPath,
				"CORE_PKG="+PkgPath,
			)
			return nil
		},
	})
}

func TestScriptCommon(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	t.Parallel()
	type testCase struct {
		name  string
		files []string
	}
	commonDir := "test/testdata/common"
	tests := []testCase{
		{"action discovery basic", []string{commonDir + "/discovery_basic.txtar"}},
		{"action discovery config naming", []string{commonDir + "/discovery_config_naming.txtar"}},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			testscript.Run(t, testscript.Params{
				Files:               tt.files,
				RequireExplicitExec: true,
				RequireUniqueNames:  true,
				ContinueOnError:     true,
			})
		})
	}

}
