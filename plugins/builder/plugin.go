// Package builder implements a plugin to build launchr with plugins.
package builder

import (
	"bufio"
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/launchrctl/launchr/internal/launchr"
)

func init() {
	launchr.RegisterPlugin(&Plugin{})
}

// Plugin is a [launchr.Plugin] to build launchr application.
type Plugin struct {
	app launchr.App
}

// PluginInfo implements [launchr.Plugin] interface.
func (p *Plugin) PluginInfo() launchr.PluginInfo {
	return launchr.PluginInfo{
		Weight: math.MinInt,
	}
}

type builderInput struct {
	name    string
	out     string
	timeout string
	version string
	tags    []string
	plugins []string
	replace []string
	debug   bool
	nocache bool
}

// OnAppInit implements [launchr.OnAppInitPlugin] interface.
func (p *Plugin) OnAppInit(app launchr.App) error {
	p.app = app
	return nil
}

// CobraAddCommands implements [launchr.CobraPlugin] interface to provide build functionality.
func (p *Plugin) CobraAddCommands(rootCmd *launchr.Command) error {
	// Flag options.
	flags := builderInput{}

	buildCmd := &launchr.Command{
		Use:   "build",
		Short: "Builds application with specified configuration",
		RunE: func(cmd *launchr.Command, _ []string) error {
			// Don't show usage help on a runtime error.
			cmd.SilenceUsage = true
			return Execute(cmd.Context(), p.app.Streams(), &flags)
		},
	}
	// Command flags.
	buildCmd.Flags().StringVarP(&flags.name, "name", "n", p.app.Name(), `Result application name`)
	buildCmd.Flags().StringVarP(&flags.out, "output", "o", "", `Build output file, by default application name is used`)
	buildCmd.Flags().StringVar(&flags.version, "build-version", "", `Arbitrary version of application`)
	buildCmd.Flags().StringVarP(&flags.timeout, "timeout", "t", "120s", `Build timeout duration, example: 0, 100ms, 1h23m`)
	buildCmd.Flags().StringSliceVarP(&flags.tags, "tag", "", nil, `Add build tags`)
	buildCmd.Flags().StringSliceVarP(&flags.plugins, "plugin", "p", nil, `Include PLUGIN into the build with an optional version`)
	buildCmd.Flags().StringSliceVarP(&flags.replace, "replace", "r", nil, `Replace go dependency, see "go mod edit -replace"`)
	buildCmd.Flags().BoolVarP(&flags.debug, "debug", "d", false, `Include debug flags into the build to support go debugging with "delve". If not specified, debugging info is trimmed`)
	buildCmd.Flags().BoolVarP(&flags.nocache, "no-cache", "", false, `Disable the usage of cache, e.g., when using 'go get' for dependencies.`)
	rootCmd.AddCommand(buildCmd)
	return nil
}

// Generate implements [launchr.GeneratePlugin] interface.
func (p *Plugin) Generate(buildPath string, _ string) error {
	launchr.Term().Info().Println("Generating main.go file")

	// Temporary solution to build with an old version.
	// @todo remove when the new version is released.
	var imports []UsePluginInfo
	_, err := os.Stat(filepath.Join(buildPath, "plugins.go"))
	if os.IsNotExist(err) {
		imports, err = extractGenImports(filepath.Join(buildPath, "gen.go"))
		if err != nil {
			return err
		}
	}

	tpl := launchr.Template{
		Tmpl: tmplMain,
		Data: &buildVars{
			CorePkg: corePkgInfo(),
			Plugins: imports,
		},
	}
	return tpl.WriteFile(filepath.Join(buildPath, "main.go"))
}

// Deprecated: remove when the new version is deployed.
func extractGenImports(genpath string) ([]UsePluginInfo, error) {
	// Open the file
	file, err := os.Open(genpath) //nolint
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var imports []UsePluginInfo
	readingImports := false
	scanner := bufio.NewScanner(file)
	importRegex := regexp.MustCompile(`^\s*(_|\w+\s+)?"([^"]+)"`)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "import (" {
			readingImports = true
			continue
		}
		if line == ")" {
			break
		}
		// If we are in the imports section, collect the import paths
		if readingImports {
			matches := importRegex.FindStringSubmatch(line)
			if len(matches) > 1 && matches[2] != launchr.PkgPath {
				imports = append(imports, UsePluginInfo{Path: matches[2]})
			}
		}
	}

	if err = scanner.Err(); err != nil {
		return nil, err
	}

	return imports, nil
}

// Execute runs launchr and executes build of launchr.
func Execute(ctx context.Context, streams launchr.Streams, flags *builderInput) error {
	// Set build timeout.
	timeout, err := time.ParseDuration(flags.timeout)
	if err != nil {
		return err
	}
	if timeout != 0 {
		// Execute build.
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// Parse dependencies definition.
	plugins, err := parsePlugins(flags.plugins)
	if err != nil {
		return err
	}
	replace, err := parseReplace(flags.replace)
	if err != nil {
		return err
	}

	// Set output path if it's not defined.
	if len(flags.out) == 0 && len(flags.name) > 0 {
		flags.out = "./" + flags.name
	}

	opts := &BuildOptions{
		AppVersion:  launchr.Version(),
		Version:     flags.version,
		CorePkg:     corePkgInfo(),
		PkgName:     flags.name,
		ModReplace:  replace,
		Plugins:     plugins,
		Tags:        flags.tags,
		BuildOutput: flags.out,
		Debug:       flags.debug,
		NoCache:     flags.nocache,
	}

	if err = opts.Validate(); err != nil {
		return err
	}

	builder, err := NewBuilder(opts)
	if err != nil {
		return err
	}
	defer builder.Close()
	return builder.Build(ctx, streams)
}

func corePkgInfo() UsePluginInfo {
	return UsePluginInfo{Path: launchr.PkgPath}
}

func parsePlugins(plugins []string) ([]UsePluginInfo, error) {
	// Collect unique plugins.
	unique := map[string]UsePluginInfo{}
	for _, pdef := range plugins {
		pi := UsePluginInfoFromString(pdef)
		unique[pi.Path] = pi
	}
	res := make([]UsePluginInfo, 0, len(unique))
	for _, p := range unique {
		res = append(res, p)
	}
	return res, nil
}

func parseReplace(replace []string) (map[string]string, error) {
	// Replace module dependencies, e.g. with local paths for development or different version.
	repl := map[string]string{}
	for _, rdef := range replace {
		oldnew := strings.SplitN(rdef, "=", 2)
		if len(oldnew) == 1 {
			return nil, fmt.Errorf("incorrect replace definition: %s", rdef)
		}
		repl[oldnew[0]] = oldnew[1]
	}
	return repl, nil
}
