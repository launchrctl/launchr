package test

import (
	"testing"

	"github.com/rogpeppe/go-internal/testscript"

	"github.com/launchrctl/launchr"
)

func TestMain(m *testing.M) {
	testscript.Main(m, map[string]func(){
		"launchr": launchr.RunAndExit,
	})
}

// TestBuild tests how binary builds and outputs version.
func TestBuild(t *testing.T) {
	t.Parallel()
	testscript.Run(t, testscript.Params{
		Dir:                 "testdata/build",
		RequireExplicitExec: true,
		RequireUniqueNames:  true,
		Setup: func(env *testscript.Env) error {
			repoPath := launchr.MustAbs("../")
			env.Vars = append(
				env.Vars,
				"REPO_PATH="+repoPath,
				"CORE_PKG="+launchr.PkgPath,
			)
			return nil
		},
	})
}

func TestCommon(t *testing.T) {
	t.Parallel()
	testscript.Run(t, testscript.Params{
		Dir:                 "testdata/common",
		RequireExplicitExec: true,
		RequireUniqueNames:  true,
		ContinueOnError:     true,
	})
}
