package builder

import (
	"bytes"
	"context"
	"embed"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
	"time"

	"github.com/launchrctl/launchr"
)

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
	dep := p.Package
	if p.Version != "" {
		dep += "@" + p.Version
	}
	return dep
}

// BuildOptions stores launchr build parameters.
type BuildOptions struct {
	LaunchrVersion *launchr.AppVersion
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

//go:embed tpl/*.tmpl
var embedTmplFs embed.FS
var tmplView = template.Must(template.ParseFS(embedTmplFs, "tpl/*.tmpl"))

type buildVars struct {
	BuildTags      string
	PkgName        string
	LaunchrVersion *launchr.AppVersion
	BuildVersion   *launchr.AppVersion
	Plugins        []UsePluginInfo
	ExecFn         string
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
		PkgName:        b.PkgName,
		BuildVersion:   buildVer,
		Plugins:        b.Plugins,
	}
	genVars := mainVars
	genVars.BuildTags = "ignore"
	genVars.ExecFn = "launchr.Gen()"
	files := []genGoFile{
		{"main.tmpl", &mainVars, "main.go"},
		{"main.tmpl", &genVars, "gen.go"},
		{"genpkg.tmpl", nil, "gen/pkg.go"},
	}

	// Write files to dir and generate go mod.
	log.Printf("[INFO] Creating project files and fetching dependencies")
	err = b.env.CreateProject(ctx, files, b.BuildOptions)
	if err != nil {
		return err
	}

	// Generate code for provided plugins.
	genArgs := []string{"run", "gen.go", b.wd}
	cmdGen := b.env.NewCommand(ctx, b.env.Go(), genArgs...)
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

func (b *Builder) getBuildVersion(ctx context.Context, version *launchr.AppVersion) *launchr.AppVersion {
	bv := *version
	bv.Name = b.PkgName
	bv.BuildDate = time.Now().Format(time.RFC3339)
	// @todo get version from the fetched go.mod module

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
