// Package builder implements launchr functionality to build itself.
package builder

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/launchrctl/launchr/pkg/log"
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

func (env *buildEnvironment) CreateModFile(ctx context.Context, opts *BuildOptions) error {
	var err error
	// Create go.mod.
	err = env.execGoMod(ctx, "init", opts.PkgName)
	if err != nil {
		return err
	}

	// Replace requested modules.
	for o, n := range opts.ModReplace {
		err = env.execGoMod(ctx, "edit", "-replace", o+"="+n)
		if err != nil {
			return err
		}
	}

	// Download core.
	var coreRepl bool
	for repl := range opts.ModReplace {
		if strings.HasPrefix(opts.CorePkg.Path, repl) {
			coreRepl = true
			break
		}
	}
	if !coreRepl {
		err = env.execGoGet(ctx, opts.CorePkg.String())
		if err != nil {
			return err
		}
	}

	// Download plugins.
nextPlugin:
	for _, p := range opts.Plugins {
		// Do not get plugins of module subpath.
		for repl := range opts.ModReplace {
			if strings.HasPrefix(p.Path, repl) {
				continue nextPlugin
			}
		}
		err = env.execGoGet(ctx, p.String())
		if err != nil {
			return err
		}
	}
	// @todo update all but with fixed versions if requested

	return err
}

func (env *buildEnvironment) CreateSourceFiles(ctx context.Context, files []genGoFile) error {
	// Generate project files.
	var buf bytes.Buffer
	var err error
	for _, f := range files {
		buf.Reset()
		// Render template.
		err = tmplView.ExecuteTemplate(&buf, f.TmplName, f.Vars)
		if err != nil {
			return err
		}
		// Create target file with directories recursively.
		target := filepath.Join(env.wd, f.Filename)
		dir := filepath.Dir(target)
		err = os.MkdirAll(dir, 0700)
		if err != nil {
			return err
		}
		err = os.WriteFile(target, buf.Bytes(), 0600)
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
	cmd.Env = env.env
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

func (env *buildEnvironment) execGoList(ctx context.Context, args ...string) (string, error) {
	cmd := env.NewCommand(ctx, env.Go(), append([]string{"list", "-m", "all"}, args...)...)
	cmd.Stdout = nil

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}

	var outputText string
	go func() {
		outputBytes, _ := io.ReadAll(stdout)
		outputText = string(outputBytes)
	}()

	err = env.RunCmd(ctx, cmd)
	return outputText, err
}

func (env *buildEnvironment) RunCmd(ctx context.Context, cmd *exec.Cmd) error {
	log.Debug("Executing shell: %s", cmd)
	log.Debug("Shell env variables: %s", cmd.Env)
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
