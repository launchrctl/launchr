package builder

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/launchrctl/launchr/internal/launchr"
)

// Builder is the orchestrator to fetch dependencies and build launchr.
type Builder struct {
	*BuildOptions
	wd  string
	env *buildEnvironment
}

// UsePluginInfo stores plugin info.
type UsePluginInfo struct {
	Path    string
	Version string
}

// UsePluginInfoFromString constructs mod plugin info.
func UsePluginInfoFromString(s string) UsePluginInfo {
	pv := strings.SplitN(s, "@", 2)
	if len(pv) == 1 {
		pv = append(pv, "")
	}
	return UsePluginInfo{pv[0], pv[1]}
}

func (p UsePluginInfo) String() string {
	dep := p.Path
	if p.Version != "" {
		dep += "@" + p.Version
	}
	return dep
}

// BuildOptions stores launchr build parameters.
type BuildOptions struct {
	AppVersion  *launchr.AppVersion
	Version     string
	CorePkg     UsePluginInfo
	PkgName     string
	ModReplace  map[string]string
	Plugins     []UsePluginInfo
	BuildOutput string
	Debug       bool
	Tags        []string
	NoCache     bool
}

var validPkgNameRegex = regexp.MustCompile(`^[a-zA-Z0-9]+$`)

// Validate verifies build options.
func (opts *BuildOptions) Validate() error {
	if !validPkgNameRegex.MatchString(opts.PkgName) {
		return fmt.Errorf(`invalid application name "%s"`, opts.PkgName)
	}
	return nil
}

type genGoFile struct {
	launchr.Template
	file string
}

type buildVars struct {
	PkgName string
	CorePkg UsePluginInfo
	Plugins []UsePluginInfo
	Cwd     string
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
func (b *Builder) Build(ctx context.Context, streams launchr.Streams) error {
	launchr.Term().Info().Printfln("Starting to build %s", b.PkgName)
	// Prepare build environment dir and go executable.
	var err error
	b.env, err = newBuildEnvironment(streams)
	if err != nil {
		return err
	}

	// Delete temp files in case of error.
	defer func() {
		if err != nil {
			_ = b.Close()
		}
	}()
	launchr.Log().Debug("creating build environment", "temp_dir", b.env.wd, "env", b.env.env)

	// Write files to dir and generate go mod.
	launchr.Term().Info().Println("Creating the project files and fetching dependencies")
	b.env.SetEnv("CGO_ENABLED", "0")
	err = b.env.CreateModFile(ctx, b.BuildOptions)
	if err != nil {
		return err
	}

	// Generate project files.
	mainVars := buildVars{
		CorePkg: b.CorePkg,
		PkgName: b.PkgName,
		Plugins: b.Plugins,
		Cwd:     b.wd,
	}
	files := []genGoFile{
		// Create files that will generate the main code.
		// See [Plugin.Generate] in plugin.go for main.go file generation.
		{launchr.Template{Tmpl: tmplPlugins, Data: &mainVars}, "plugins.go"},
		{launchr.Template{Tmpl: tmplGen, Data: &mainVars}, "gen.go"},
	}

	launchr.Term().Info().Println("Generating the go files")
	for _, f := range files {
		// Generate the file.
		err = f.WriteFile(filepath.Join(b.env.wd, f.file))
		if err != nil {
			return err
		}
	}

	// Generate code for provided plugins.
	launchr.Term().Info().Println("Running plugin generation")
	err = b.runGoRun(ctx, b.env.wd, "gen.go", "--work-dir="+b.wd, "--build-dir="+b.env.wd, "--release")
	if err != nil {
		return err
	}

	// Build the main go package.
	launchr.Term().Info().Printfln("Building %s", b.PkgName)
	err = b.goBuild(ctx)
	if err != nil {
		return err
	}

	launchr.Term().Success().Printfln("Build complete: %s", b.BuildOutput)
	return nil
}

// Close does cleanup after build.
func (b *Builder) Close() error {
	if b.env != nil && !b.Debug {
		launchr.Log().Debug("cleaning build environment directory", "dir", b.env.wd)
		return b.env.Close()
	}
	return nil
}

func (b *Builder) goBuild(ctx context.Context) error {
	out, err := filepath.Abs(b.BuildOutput)
	if err != nil {
		return err
	}
	// Set build result file.
	args := []string{"build", "-o", out}
	// Collect ldflags
	ldflags := make([]string, 0, 5)
	// Set application version metadata.
	ldflags = append(
		ldflags,
		"-X", "'"+launchr.PkgPath+".name="+b.PkgName+"'",
		"-X", "'"+launchr.PkgPath+".builtWith="+b.AppVersion.Short()+"'",
	)

	if b.Version != "" {
		ldflags = append(ldflags, "-X", "'"+launchr.PkgPath+".version="+b.Version+"'")
	}

	// Include or trim debug information.
	if b.Debug {
		args = append(args, "-gcflags", "all=-N -l")
	} else {
		ldflags = append(ldflags, "-s", "-w")
		args = append(args, "-trimpath")
	}
	args = append(args, "-ldflags", strings.Join(ldflags, " "))

	if len(b.Tags) > 0 {
		args = append(args, "-tags", strings.Join(b.Tags, " "))
	}

	// Run build.
	cmd := b.env.NewCommand(ctx, b.env.Go(), args...)
	err = b.env.RunCmd(ctx, cmd)
	if err != nil {
		return err
	}

	return nil
}

func (b *Builder) runGoRun(ctx context.Context, dir string, args ...string) error {
	runArgs := append([]string{"run"}, args...)
	cmd := b.env.NewCommand(ctx, b.env.Go(), runArgs...)
	cmd.Dir = dir
	env := make(envVars, len(cmd.Env))
	copy(env, cmd.Env)
	// Exclude target platform information as it may break "go run".
	env.Unset("GOOS")
	env.Unset("GOARCH")
	cmd.Env = env
	return b.env.RunCmd(ctx, cmd)
}
