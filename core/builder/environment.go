// Package builder implements launchr functionality to build itself.
package builder

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type envVars []string

func envFromOs() envVars {
	osenv := os.Environ()
	env := make(envVars, 0, 8)
	for _, v := range osenv {
		if strings.HasPrefix(v, "GO") || strings.HasPrefix(v, "HOME=") {
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
			(*a)[i] = fmt.Sprintf("%s=%s", k, v)
			return
		}
	}
	*a = append(*a, fmt.Sprintf("%s=%s", k, v))
}

type buildEnvironment struct {
	wd  string
	env envVars
}

func newBuildEnvironment() (*buildEnvironment, error) {
	tmpDir, err := os.MkdirTemp(".", "build_")
	if err != nil {
		return nil, err
	}

	return &buildEnvironment{wd: tmpDir, env: envFromOs()}, nil
}

func (env *buildEnvironment) CreateProject(ctx context.Context, files []genGoFile, opts *BuildOptions) error {
	// Generate project files.
	var buf bytes.Buffer
	var err error
	for _, f := range files {
		buf.Reset()
		err = f.Tpl.Execute(&buf, f.Vars)
		if err != nil {
			return err
		}
		target := filepath.Join(env.wd, f.Filename)
		err = os.WriteFile(target, buf.Bytes(), 0600)
		if err != nil {
			return err
		}
	}

	// Create go.mod.
	err = env.execGoMod(ctx, "init", "launchr")
	if err != nil {
		return err
	}

	// Replace requested modules.
	for o, n := range opts.ModReplace {
		repl := fmt.Sprintf("%s=%s", o, n)
		err = env.execGoMod(ctx, "edit", "-replace", repl)
		if err != nil {
			return err
		}
	}

	// Download plugins.
nextPlugin:
	for _, p := range opts.Plugins {
		// Do not get plugins of module subpath.
		for repl := range opts.ModReplace {
			if strings.HasPrefix(p.Package, repl) {
				continue nextPlugin
			}
		}
		err = env.execGoGet(ctx, p.String())
		if err != nil {
			return err
		}
	}

	// Make sure all dependencies are met.
	err = env.execGoMod(ctx, "tidy")
	if err != nil {
		return err
	}

	return err
}

func (env *buildEnvironment) Filepath(s string) string {
	return filepath.Join(env.wd, s)
}

func (env *buildEnvironment) NewCommand(ctx context.Context, command string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = env.wd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

func (env *buildEnvironment) execGoMod(ctx context.Context, args ...string) error {
	cmd := env.NewCommand(ctx, env.Go(), append([]string{"mod"}, args...)...)
	return env.RunCmd(ctx, cmd)
}

func (env *buildEnvironment) execGoGet(ctx context.Context, args ...string) error {
	cmd := env.NewCommand(ctx, env.Go(), append([]string{"get", "-d"}, args...)...)
	return env.RunCmd(ctx, cmd)
}

func (env *buildEnvironment) RunCmd(ctx context.Context, cmd *exec.Cmd) error {
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
