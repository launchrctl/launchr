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

	"golang.org/x/mod/modfile"

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
			(*a)[i] = fmt.Sprintf("%s=%s", k, v)
			return
		}
	}
	*a = append(*a, fmt.Sprintf("%s=%s", k, v))
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
	wd    string
	env   envVars
	gomod *modfile.File
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
		repl := fmt.Sprintf("%s=%s", o, n)
		err = env.execGoMod(ctx, "edit", "-replace", repl)
		if err != nil {
			return err
		}
	}

	err = env.execGoGet(ctx, opts.CorePkg.GoGetString())
	if err != nil {
		return err
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
		err = env.execGoGet(ctx, p.GoGetString())
		if err != nil {
			return err
		}
	}

	err = env.parseGoMod()
	if err != nil {
		return err
	}

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

func (env *buildEnvironment) parseGoMod() error {
	var err error
	modFile, err := os.ReadFile(filepath.Join(env.wd, "go.mod"))
	if err != nil {
		return err
	}
	env.gomod, err = modfile.Parse("go.mod", modFile, nil)
	if err != nil {
		return err
	}
	return nil
}

func (env *buildEnvironment) GetPkgVersion(pkg string) string {
	var vrep *modfile.Replace
	for _, rep := range env.gomod.Replace {
		if strings.HasPrefix(pkg, rep.Old.Path) {
			vrep = rep
			break
		}
	}

	var vreq *modfile.Require
	for _, req := range env.gomod.Require {
		if strings.HasPrefix(pkg, req.Mod.Path) {
			vreq = req
			break
		}
	}
	v := vreq.Mod.Version
	if vrep != nil && vrep.New.Version != "" {
		v = vrep.New.Version
	}
	return v
}

func (env *buildEnvironment) SetEnv(k string, v string) {
	env.env.Set(k, v)
}
