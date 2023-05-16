package builder

import (
	"bytes"
	"context"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
	"time"

	"github.com/launchrctl/launchr/core"
)

// Builder is the orchestrator to fetch dependencies and build launchr.
type Builder struct {
	*BuildOptions
	wd      string
	env     *buildEnvironment
	tplMain *template.Template
}

// UsePluginInfo stores plugin info.
type UsePluginInfo struct {
	Package string
	Version string
}

func (p UsePluginInfo) String() string {
	dep := p.Package
	if p.Version != "" {
		dep += "@" + p.Version
	}
	return dep
}

// BuildOptions stores launchr build parameters.
type BuildOptions struct {
	LaunchrVersion *core.AppVersion
	ModReplace     map[string]string
	Plugins        []UsePluginInfo
	BuildOutput    string
	Debug          bool
}

type genGoFile struct {
	Tpl      *template.Template
	Vars     interface{}
	Filename string
}

// NewBuilder creates build environment.
func NewBuilder(opts *BuildOptions) (*Builder, error) {
	tplMain, err := newAppTpl()
	if err != nil {
		return nil, err
	}
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return &Builder{
		BuildOptions: opts,
		tplMain:      tplMain,
		wd:           wd,
	}, nil
}

// Build prepares build environment, generates go files and build the binary.
func (b *Builder) Build(ctx context.Context) error {
	log.Printf("[INFO] Start building")
	// Execute build.
	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	// Prepare build environment dir and go executable.
	var err error
	b.env, err = newBuildEnvironment()
	if err != nil {
		return err
	}

	// Delete temp files in case of error.
	defer func() {
		if err != nil {
			_ = b.Close()
		}
	}()
	log.Printf("[INFO] Temporary folder: %s", b.env.wd)

	// Generate app version info.
	buildVer := b.getBuildVersion(ctx, b.LaunchrVersion)

	// Generate project files.
	mainVars := buildVars{
		LaunchrVersion: b.LaunchrVersion,
		BuildVersion:   buildVer,
		Plugins:        b.Plugins,
	}
	genVars := mainVars
	genVars.BuildTags = "ignore"
	genVars.ExecFn = "app.Generate()"
	files := []genGoFile{
		{b.tplMain, &mainVars, "main.go"},
		{b.tplMain, &genVars, "gen.go"},
	}

	// Write files to dir and generate go mod.
	log.Printf("[INFO] Creating project files and fetching dependencies")
	err = b.env.CreateProject(ctx, files, b.BuildOptions)
	if err != nil {
		return err
	}

	// Generate code for provided plugins.
	genArgs := []string{"run", filepath.Join(b.env.wd, "gen.go"), b.env.wd}
	cmdGen := b.env.NewCommand(ctx, b.env.Go(), genArgs...)
	cmdGen.Dir = b.wd
	err = b.env.RunCmd(ctx, cmdGen)
	if err != nil {
		return err
	}

	// Build the main go package.
	log.Printf("[INFO] Building Launchr")
	err = b.goBuild(ctx)
	if err != nil {
		return err
	}

	log.Printf("[INFO] Build complete: %s", b.BuildOutput)
	return nil
}

// Close does cleanup after build.
func (b *Builder) Close() error {
	if b.env != nil && !b.Debug {
		return b.env.Close()
	}
	return nil
}

func (b *Builder) goBuild(ctx context.Context) error {
	out, err := filepath.Abs(b.BuildOutput)
	if err != nil {
		return err
	}
	args := []string{
		"build",
		"-o",
		out,
	}
	if b.Debug {
		args = append(args, "-gcflags", "all=-N -l")
	} else {
		args = append(args, "-ldflags", "-w -s", "-trimpath")
	}
	//args = append(args, b.env.wd)
	cmd := b.env.NewCommand(ctx, b.env.Go(), args...)
	cmd.Env = envFromOs()

	log.Printf("[DEBUG] Go build command: %s", cmd)
	log.Printf("[DEBUG] Environment variables: %v", cmd.Env)
	err = b.env.RunCmd(ctx, cmd)
	if err != nil {
		return err
	}

	return nil
}

func (b *Builder) getBuildVersion(ctx context.Context, version *core.AppVersion) *core.AppVersion {
	bv := *version
	bv.Name = "launchr"
	bv.BuildDate = time.Now().Format(time.RFC3339)

	// Get go version that would build
	bufOut := bytes.NewBuffer(nil)
	bufErr := bytes.NewBuffer(nil)
	cmd := b.env.NewCommand(ctx, b.env.Go(), "version")
	cmd.Stdout = bufOut
	cmd.Stderr = bufErr
	err := b.env.RunCmd(ctx, cmd)
	if err == nil {
		bv.GoVersion = strings.TrimSpace(bufOut.String())[len("go version "):]
	}

	bv.OS = os.Getenv("GOOS")
	bv.Arch = os.Getenv("GOARCH")
	bv.Arm = os.Getenv("GOARM")
	if bv.OS == "" {
		bv.OS = runtime.GOOS
	}
	if bv.Arch == "" {
		bv.Arch = runtime.GOARCH
	}

	return &bv
}
