// Package builder implements a plugin to build launchr with plugins.
package builder

import (
	"context"
	"fmt"
	"math"
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
	rootCmd.AddCommand(buildCmd)
	return nil
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
		LaunchrVersion: launchr.Version(),
		Version:        flags.version,
		CorePkg:        UsePluginInfo{Path: launchr.PkgPath},
		PkgName:        flags.name,
		ModReplace:     replace,
		Plugins:        plugins,
		Tags:           flags.tags,
		BuildOutput:    flags.out,
		Debug:          flags.debug,
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
