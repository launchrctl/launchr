package builder

import (
	"context"
	"embed"
	"fmt"
	"go/build"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	cp "github.com/otiai10/copy"
	"golang.org/x/mod/module"

	"github.com/launchrctl/launchr/internal/launchr"
	"github.com/launchrctl/launchr/pkg/cli"
	"github.com/launchrctl/launchr/pkg/log"
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
	LaunchrVersion *launchr.AppVersion
	Version        string
	CorePkg        UsePluginInfo
	PkgName        string
	ModReplace     map[string]string
	Plugins        []UsePluginInfo
	BuildOutput    string
	Debug          bool
	Tags           []string
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
	TmplName string
	Vars     interface{}
	Filename string
}

//go:embed tmpl/*.tmpl
var embedTmplFs embed.FS
var tmplView = template.Must(template.ParseFS(embedTmplFs, "tmpl/*.tmpl"))

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

	// prebuild
	cli.Println("Executing prebuild scripts")
	err = b.preBuild(ctx)
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
	// Set build result file.
	args := []string{"build", "-o", out}
	// Collect ldflags
	ldflags := make([]string, 0, 5)
	// Set application version metadata.
	ldflags = append(
		ldflags,
		"-X", "'"+launchr.PkgPath+".name="+b.PkgName+"'",
		"-X", "'"+launchr.PkgPath+".builtWith="+b.LaunchrVersion.Short()+"'",
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

func (b *Builder) preBuild(ctx context.Context) error {
	output, err := b.env.execGoList(ctx)
	if err != nil {
		return err
	}

	pluginVersionMap := make(map[string]string)
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		pv := strings.Split(line, " ")
		for _, p := range b.BuildOptions.Plugins {
			if strings.Contains(pv[0], p.Path) {
				pluginVersionMap[p.Path] = pv[1]
				continue
			}
		}
	}

	assetsPath := filepath.Join(b.wd, b.env.wd, "assets")
	buildName := filepath.Base(b.env.wd)
	err = os.MkdirAll(assetsPath, 0750)
	if err != nil {
		return err
	}

	for pluginName, v := range pluginVersionMap {
		if _, ok := b.BuildOptions.ModReplace[pluginName]; ok {
			log.Debug("skipping prebuild script for replaced plugin %s", pluginName)
			continue
		}

		packagePath, _ := getModulePath(pluginName, v)
		if _, err = os.Stat(packagePath); os.IsNotExist(err) {
			log.Debug("you don't have this module/version installed (%s)", packagePath)
			continue
		}

		// check if prebuild script exists.
		prebuildScriptPath := filepath.Join(packagePath, "scripts", "prebuild.go")
		if _, err = os.Stat(prebuildScriptPath); os.IsNotExist(err) {
			log.Debug("prebuild script does not exist for %s, skipping", pluginName)
			continue
		}

		tmpPath := filepath.Join(os.TempDir(), buildName, filepath.Base(pluginName))

		// clean tmp folder if it existed before.
		err = os.RemoveAll(tmpPath)
		if err != nil {
			return err
		}

		// prepare tmp folder for assets and force prebuild script to push data there.
		err = os.MkdirAll(tmpPath, 0750)
		if err != nil {
			return err
		}

		log.Debug("executing prebuild script for plugin %s", pluginName)
		err = b.runGoRun(ctx, packagePath, []string{"scripts/prebuild.go", v, tmpPath})
		if err != nil {
			return err
		}

		// prepare plugin assets folder.
		pluginAssetsPath := filepath.Join(assetsPath, pluginName)
		err = os.MkdirAll(pluginAssetsPath, 0750)
		if err != nil {
			return err
		}

		// move assets from tmp dir to assets folder.
		log.Debug("moving assets from tmp to build folder")
		err = cp.Copy(tmpPath, pluginAssetsPath, cp.Options{OnDirExists: func(_, _ string) cp.DirExistsAction {
			return cp.Merge
		}})
		if err != nil {
			return err
		}

		log.Debug("removing tmp files")
		err = os.RemoveAll(tmpPath)
		if err != nil {
			return err
		}
	}

	err = os.RemoveAll(filepath.Join(os.TempDir(), buildName))
	if err != nil {
		return err
	}

	// create empty .info file for embed.
	// prevent embed error in case of 0 assets in folder.
	file, err := os.Create(filepath.Clean(filepath.Join(assetsPath, ".info")))
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(2)
	}
	defer file.Close()

	return nil
}

func (b *Builder) runGoRun(ctx context.Context, dir string, args []string) error {
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

func getModulePath(name, version string) (string, error) {
	cache, ok := os.LookupEnv("GOMODCACHE")
	if !ok {
		gopath := os.Getenv("GOPATH")
		if gopath == "" {
			gopath = build.Default.GOPATH
		}

		cache = path.Join(gopath, "pkg", "mod")
	}

	escapedPath, err := module.EscapePath(name)
	if err != nil {
		return "", err
	}

	escapedVersion, err := module.EscapeVersion(version)
	if err != nil {
		return "", err
	}

	return path.Join(cache, escapedPath+"@"+escapedVersion), nil
}
