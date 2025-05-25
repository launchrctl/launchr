package launchr

import (
	"runtime"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
)

func TestMain(m *testing.M) {
	testscript.Main(m, map[string]func(){
		"launchr": RunAndExit,
	})
}

// TestScriptBuild tests how binary builds and outputs version.
func TestScriptBuild(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}
	if runtime.GOOS == "windows" {
		t.Skip("skipping test on Windows")
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
	t.Parallel()
	testscript.Run(t, testscript.Params{
		Dir:                 "test/testdata/common",
		RequireExplicitExec: true,
		RequireUniqueNames:  true,
		ContinueOnError:     true,
	})
}
