// Package builder implements launchr functionality to build itself.
package builder

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/launchrctl/launchr/internal/launchr"
	"github.com/launchrctl/launchr/pkg/action"
)

type envVars []string

func envFromOs() envVars {
	osenv := os.Environ()
	env := make(envVars, 0, 8)
	for _, v := range osenv {
		if strings.HasPrefix(v, "GO") ||
			strings.HasPrefix(v, "HOME=") ||
			strings.HasPrefix(v, "PATH=") {
			env = append(env, v)
		}
	}
	return env
}

func (a *envVars) Set(k string, v string) {
	if k == "" || v == "" {
		return
	}
	for i := 0; i < len(*a); i++ {
		if strings.HasPrefix((*a)[i], k+"=") {
			(*a)[i] = k + "=" + v
			return
		}
	}
	*a = append(*a, k+"="+v)
}

func (a *envVars) Unset(k string) {
	if k == "" {
		return
	}
	for i := 0; i < len(*a); i++ {
		if strings.HasPrefix((*a)[i], k+"=") {
			*a = append((*a)[:i], (*a)[i+1:]...)
			return
		}
	}
}

type buildEnvironment struct {
	action.WithLogger
	action.WithTerm

	wd      string
	env     envVars
	streams launchr.Streams
}

func newBuildEnvironment(streams launchr.Streams) (*buildEnvironment, error) {
	tmpDir, err := launchr.MkdirTemp("build_")
	if err != nil {
		return nil, err
	}
	tmpDir, err = filepath.Abs(tmpDir)
	if err != nil {
		return nil, err
	}

	env := envFromOs()
	return &buildEnvironment{
		wd:      tmpDir,
		env:     env,
		streams: streams,
	}, nil
}

// ensureModuleRequired adds a module to go.mod if it's not already there or if it's replaced.
// It uses a placeholder version for replaced modules.
func (env *buildEnvironment) ensureModuleRequired(ctx context.Context, modulePath string, modReplace map[string]string) error {
	// Check if the module is replaced (exact match).
	replaced := false
	for replPath := range modReplace {
		if modulePath == replPath {
			replaced = true
			break
		}
	}

	pkgStr := modulePath
	if replaced {
		// If module is replaced, use a placeholder version for `go mod edit -require`.
		// Ensure it has a version, even if it's a placeholder.
		if !strings.Contains(pkgStr, "@") {
			pkgStr += "@v0.0.0"
		}
	}

	// Use `go mod edit -require` to ensure the module is in go.mod.
	// This command handles cases where the module is already required or needs to be added.
	// If it's a replaced module, it will ensure the replacement is respected.
	err := env.execGoMod(ctx, "edit", "-require", pkgStr)
	if err != nil {
		return err
	}
	return nil
}

func (env *buildEnvironment) CreateModFile(ctx context.Context, opts *BuildOptions) error {
	var err error
	// Create go.mod.
	err = env.execGoMod(ctx, "init", opts.PkgName)
	if err != nil {
		return err
	}

	// Apply requested module replacements.
	for o, n := range opts.ModReplace {
		err = env.execGoMod(ctx, "edit", "-replace", o+"="+n)
		if err != nil {
			return err
		}
	}

	// Download dependencies.
	if opts.NoCache {
		// Set GONOSUMDB and GONOPROXY for modules that should not be cached or verified.
		// This is typically used for local development or specific build scenarios.
		domains := make([]string, len(opts.Plugins))
		for i, p := range opts.Plugins {
			domains[i] = p.Path
		}
		// Add core package path to the list if it's not already there
		if !strings.Contains(strings.Join(domains, ","), opts.CorePkg.Path) {
			domains = append(domains, opts.CorePkg.Path)
		}
		noproxy := strings.Join(domains, ",")
		env.env = append(env.env, "GONOSUMDB="+noproxy, "GONOPROXY="+noproxy)
	}

	// Ensure core package is required.
	err = env.ensureModuleRequired(ctx, opts.CorePkg.String(), opts.ModReplace)
	if err != nil {
		return err
	}

	// Ensure plugins are required.
	for _, p := range opts.Plugins {
		// Skip plugins that are subpaths of replaced modules.
		isSubpath := false
		for repl := range opts.ModReplace {
			if p.Path != repl && strings.HasPrefix(p.Path, repl) {
				isSubpath = true
				break
			}
		}
		if isSubpath {
			continue
		}

		// Ensure the plugin module is required in go.mod.
		err = env.ensureModuleRequired(ctx, p.String(), opts.ModReplace)
		if err != nil {
			return err
		}
	}

	// @todo update all but with fixed versions if requested

	return nil
}

func (env *buildEnvironment) Filepath(s string) string {
	return filepath.Join(env.wd, s)
}

func (env *buildEnvironment) NewCommand(ctx context.Context, command string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = env.wd
	cmd.Env = env.env
	cmd.Stdout = env.streams.Out()
	cmd.Stderr = env.streams.Err()
	return cmd
}

func (env *buildEnvironment) execGoMod(ctx context.Context, args ...string) error {
	cmd := env.NewCommand(ctx, env.Go(), append([]string{"mod"}, args...)...)
	return env.RunCmd(ctx, cmd)
}

func (env *buildEnvironment) execGoGet(ctx context.Context, args ...string) error {
	cmd := env.NewCommand(ctx, env.Go(), append([]string{"get"}, args...)...)
	return env.RunCmd(ctx, cmd)
}

func (env *buildEnvironment) RunCmd(ctx context.Context, cmd *exec.Cmd) error {
	env.Log().Debug("executing shell", "cmd", cmd)
	err := cmd.Start()
	if err != nil {
		return err
	}

	// Wait for the build.
	cmdErrChan := make(chan error)
	go func() {
		cmdErrChan <- cmd.Wait()
	}()

	select {
	case cmdErr := <-cmdErrChan:
		return cmdErr
	case <-ctx.Done():
		select {
		case <-time.After(15 * time.Second):
			_ = cmd.Process.Kill()
		case <-cmdErrChan:
		}
		return ctx.Err()
	}
}

func (env *buildEnvironment) Go() string {
	return "go"
}

func (env *buildEnvironment) Close() error {
	return os.RemoveAll(env.wd)
}

func (env *buildEnvironment) SetEnv(k string, v string) {
	env.env.Set(k, v)
}
