// Package builder implements launchr functionality to build itself.
package builder

import (
	"context"
	"io"
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

	wd  string
	env envVars
}

func newBuildEnvironment(b *Builder) (*buildEnvironment, error) {
	tmpDir, err := launchr.MkdirTemp("build_", b.Debug)
	if err != nil {
		return nil, err
	}
	tmpDir, err = filepath.Abs(tmpDir)
	if err != nil {
		return nil, err
	}

	env := &buildEnvironment{
		wd:  tmpDir,
		env: envFromOs(),
	}
	env.SetLogger(b.Log())
	env.SetTerm(b.Term())
	return env, nil
}

// ensureModuleRequired adds a replaced module to go.mod as a requirement.
// Replaced modules need an explicit require directive with a placeholder version,
// otherwise Go reports "replaced but not required" errors during compilation.
func (env *buildEnvironment) ensureModuleRequired(ctx context.Context, pkg string) error {
	// Strip version suffix if present, use placeholder version for replaced modules.
	mod, _, _ := strings.Cut(pkg, "@")
	return env.execGoMod(ctx, "edit", "-require", mod+"@v0.0.0")
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
		domains := make([]string, 0, len(opts.Plugins)+1)
		for _, p := range opts.Plugins {
			domains = append(domains, p.Path)
		}
		if opts.CorePkg.Path != "" {
			domains = append(domains, opts.CorePkg.Path)
		}
		noproxy := strings.Join(domains, ",")
		env.env = append(env.env, "GONOSUMDB="+noproxy, "GONOPROXY="+noproxy)
	}

	// Download core package.
	// Replaced modules need an explicit require directive first, otherwise Go
	// reports "replaced but not required" errors during compilation.
	if _, ok := opts.ModReplace[opts.CorePkg.Path]; ok {
		if err = env.ensureModuleRequired(ctx, opts.CorePkg.String()); err != nil {
			return err
		}
	}
	if err = env.execGoGet(ctx, opts.CorePkg.String()); err != nil {
		return err
	}

	// Download plugins.
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

		// Replaced modules need an explicit require directive before go get.
		if _, ok := opts.ModReplace[p.Path]; ok {
			if err = env.ensureModuleRequired(ctx, p.String()); err != nil {
				return err
			}
		}
		if err = env.execGoGet(ctx, p.String()); err != nil {
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
	cmd.Stdout = env.Term()
	cmd.Stderr = env.Term()
	return cmd
}

func (env *buildEnvironment) execGoMod(ctx context.Context, args ...string) error {
	cmd := env.NewCommand(ctx, env.Go(), append([]string{"mod"}, args...)...)
	// Don't output go output unless some verbosity is requested.
	if env.Log().Level() != launchr.LogLevelDebug {
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
	}
	return env.RunCmd(ctx, cmd)
}

func (env *buildEnvironment) execGoGet(ctx context.Context, args ...string) error {
	cmd := env.NewCommand(ctx, env.Go(), append([]string{"get"}, args...)...)
	// Don't output go output unless some verbosity is requested.
	if env.Log().Level() == launchr.LogLevelDisabled {
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
	}
	return env.RunCmd(ctx, cmd)
}

func (env *buildEnvironment) RunCmd(ctx context.Context, cmd *exec.Cmd) error {
	env.Log().Debug("executing shell", "cmd", cmd, "pwd", cmd.Dir)
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
