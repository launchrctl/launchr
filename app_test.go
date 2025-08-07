package launchr

import (
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rogpeppe/go-internal/testscript"

	"github.com/launchrctl/launchr/internal/launchr"
	coretest "github.com/launchrctl/launchr/test"
)

func TestMain(m *testing.M) {
	// Set testscript version.
	version = "v0.0.0-testscript"
	builtWith = "testscript v0.0.0"
	testscript.Main(m, map[string]func(){
		"launchr": RunAndExit,
		"testapp": func() {
			// Set global application name.
			name = "testapp"
			RunAndExit()
		},
	})
}

func TestBinary(t *testing.T) {
	t.Parallel()

	type tsSetupFn = func(*testscript.Env) error
	type reqFn = func(t *testing.T)
	type testcase struct {
		name   string
		dir    string
		files  []string
		setup  []tsSetupFn
		req    []reqFn
		conseq bool
	}

	supportedOS := func(os ...string) reqFn {
		return func(t *testing.T) {
			if !slices.Contains(os, runtime.GOOS) {
				t.Skipf("skipping %q, supported os: %s", runtime.GOOS, strings.Join(os, ", "))
			}
		}
	}
	skipShort := func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping test in short mode")
		}
	}
	hasWSL := func(t *testing.T) {
		if !isWSLAvailable() {
			t.Skip("skipping test on Windows without WSL")
		}
	}

	testcases := []testcase{
		{name: "common", dir: "test/testdata/common"},
		{name: "action/discovery", dir: "test/testdata/action/discovery"},
		{name: "action/input", dir: "test/testdata/action/input"},

		// Runtime Shell on Unix os.
		{
			name: "runtime/shell/unix",
			dir:  "test/testdata/runtime/shell",
			req:  []reqFn{supportedOS("linux", "darwin")},
		},
		// Runtime Shell on Windows MSYS.
		// To test this on Windows, make sure that the MSYS-like bash is the first in the PATH.
		{
			name:  "runtime/shell/win-msys",
			dir:   "test/testdata/runtime/shell",
			req:   []reqFn{supportedOS("windows")},
			setup: []tsSetupFn{coretest.SetupWorkDirUnixWin},
		},
		// Runtime Shell using Windows WSL.
		// To test this on Windows, make sure that the WSL bash is the first in the PATH.
		{
			name:  "runtime/shell/win-wsl",
			dir:   "test/testdata/runtime/shell",
			req:   []reqFn{supportedOS("windows"), hasWSL},
			setup: []tsSetupFn{coretest.SetupWSL(t)},
		},

		// Runtime Docker.
		{
			name:  "runtime/container/docker",
			dir:   "test/testdata/runtime/container",
			setup: []tsSetupFn{coretest.SetupEnvDocker, coretest.SetupEnvRandom},
			req:   []reqFn{skipShort},
		},

		// Test binary build using self.
		// This test must run last and should not be parallelized
		// so that the build cache is warm after it.
		// If it fails due to a timeout, try warming the cache manually with `make build`.
		{
			// Run the build once to warm up the build cache.
			name:   "build-warmup",
			files:  []string{"test/testdata/build/no-cache.txtar"},
			setup:  []tsSetupFn{setupBuildEnv},
			req:    []reqFn{skipShort, supportedOS("linux", "darwin")},
			conseq: true,
		},
		{
			name:  "build",
			dir:   "test/testdata/build",
			setup: []tsSetupFn{setupBuildEnv},
			req:   []reqFn{skipShort, supportedOS("linux", "darwin")},
		},
	}
	for _, tt := range testcases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			for _, fn := range tt.req {
				fn(t)
			}
			t.TempDir()
			if !tt.conseq {
				t.Parallel()
			}
			var deadline time.Time
			if !launchr.Version().Debug {
				deadline = time.Now().Add(5 * time.Minute)
			}

			testscript.Run(t, testscript.Params{
				Dir:      tt.dir,
				Files:    tt.files,
				Cmds:     coretest.CmdsTestScript(),
				Deadline: deadline,

				RequireExplicitExec: true,
				RequireUniqueNames:  true,

				Setup: func(env *testscript.Env) error {
					for _, fn := range tt.setup {
						if err := fn(env); err != nil {
							return err
						}
					}
					return nil
				},
			})
		})
	}
}

func setupBuildEnv(env *testscript.Env) error {
	repoPath := MustAbs("./")
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	env.Vars = append(
		env.Vars,
		"REPO_PATH="+repoPath,
		"CORE_PKG="+PkgPath,
		"REAL_HOME="+home,
	)
	return nil
}

func isWSLAvailable() bool {
	if runtime.GOOS != "windows" {
		return false
	}

	// Check if WSL command exists
	if _, err := exec.LookPath("wsl"); err != nil {
		return false
	}

	// Test if WSL is functional
	cmd := exec.Command("wsl", "echo", "test")
	return cmd.Run() == nil
}
