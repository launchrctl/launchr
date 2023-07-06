package builder

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"text/template"

	"github.com/launchrctl/launchr"
	"github.com/launchrctl/launchr/pkg/cli"
	"github.com/launchrctl/launchr/pkg/log"
)

const launchrPkg = "github.com/launchrctl/launchr"

// Builder is the orchestrator to fetch dependencies and build launchr.
type Builder struct {
	*BuildOptions
	wd  string
	env *buildEnvironment
}

// UsePluginInfo stores plugin info.
type UsePluginInfo struct {
	Package string
	Version string
}

func (p UsePluginInfo) String() string {
	return fmt.Sprintf("%s %s", p.Package, p.Version)
}

// GoGetString provides a package path for a go get.
func (p UsePluginInfo) GoGetString() string {
	dep := p.Package
	if p.Version != "" {
		dep += "@" + p.Version
	}
	return dep
}

// BuildOptions stores launchr build parameters.
type BuildOptions struct {
	LaunchrVersion *launchr.AppVersion
	CorePkg        UsePluginInfo
	PkgName        string
	ModReplace     map[string]string
	Plugins        []UsePluginInfo
	BuildOutput    string
	Debug          bool
}

type genGoFile struct {
	TmplName string
	Vars     interface{}
	Filename string
}

//go:embed tmpl/*.tmpl
var embedTmplFs embed.FS
var tmplView = template.Must(template.ParseFS(embedTmplFs, "tmpl/*.tmpl"))

type buildVars struct {
	PkgName        string
	LaunchrVersion *launchr.AppVersion
	CorePkg        UsePluginInfo
	BuildVersion   *launchr.AppVersion
	Plugins        []UsePluginInfo
	Cwd            string
}

// NewBuilder creates build environment.
func NewBuilder(opts *BuildOptions) (*Builder, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return &Builder{
		BuildOptions: opts,
		wd:           wd,
	}, nil
}

// Build prepares build environment, generates go files and build the binary.
func (b *Builder) Build(ctx context.Context) error {
	cli.Println("Starting building %s", b.PkgName)
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
	log.Debug("Temporary folder: %s", b.env.wd)

	// Write files to dir and generate go mod.
	cli.Println("Creating project files and fetching dependencies")
	b.env.SetEnv("CGO_ENABLE", "0")
	err = b.env.CreateModFile(ctx, b.BuildOptions)
	if err != nil {
		return err
	}

	// Generate app version info.
	buildVer := b.getBuildVersion(b.LaunchrVersion)
	for _, p := range b.Plugins {
		if p.Version == "" {
			p.Version = b.env.GetPkgVersion(p.Package)
		}
	}

	// Generate project files.
	mainVars := buildVars{
		LaunchrVersion: b.LaunchrVersion,
		CorePkg:        b.CorePkg,
		PkgName:        b.PkgName,
		BuildVersion:   buildVer,
		Plugins:        b.Plugins,
		Cwd:            b.wd,
	}
	files := []genGoFile{
		{"main.tmpl", &mainVars, "main.go"},
		{"gen.tmpl", &mainVars, "gen.go"},
		{"genpkg.tmpl", nil, "gen/pkg.go"},
	}

	err = b.env.CreateSourceFiles(ctx, files)
	if err != nil {
		return err
	}

	// Generate code for provided plugins.
	err = b.runGen(ctx)
	if err != nil {
		return err
	}

	// Make sure all dependencies are met after generation.
	err = b.env.execGoMod(ctx, "tidy")
	if err != nil {
		return err
	}

	// Build the main go package.
	cli.Println("Building %s", b.PkgName)
	err = b.goBuild(ctx)
	if err != nil {
		return err
	}

	cli.Println("Build complete: %s", b.BuildOutput)
	return nil
}

// Close does cleanup after build.
func (b *Builder) Close() error {
	if b.env != nil && !b.Debug {
		log.Debug("Cleaning build environment: %s", b.env.wd)
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
	cmd := b.env.NewCommand(ctx, b.env.Go(), args...)
	err = b.env.RunCmd(ctx, cmd)
	if err != nil {
		return err
	}

	return nil
}

func (b *Builder) getBuildVersion(version *launchr.AppVersion) *launchr.AppVersion {
	bv := *version
	bv.Name = b.PkgName

	// Get version from the fetched go.mod module
	bv.Version = b.env.GetPkgVersion(b.CorePkg.Package)

	bv.OS = os.Getenv("GOOS")
	bv.Arch = os.Getenv("GOARCH")
	if bv.OS == "" {
		bv.OS = runtime.GOOS
	}
	if bv.Arch == "" {
		bv.Arch = runtime.GOARCH
	}

	return &bv
}

func (b *Builder) runGen(ctx context.Context) error {
	genArgs := []string{"generate", "./..."}
	cmd := b.env.NewCommand(ctx, b.env.Go(), genArgs...)
	env := make(envVars, len(cmd.Env))
	copy(env, cmd.Env)
	// Exclude target platform information as it may break "go run".
	env.Unset("GOOS")
	env.Unset("GOARCH")
	cmd.Env = env
	return b.env.RunCmd(ctx, cmd)
}
