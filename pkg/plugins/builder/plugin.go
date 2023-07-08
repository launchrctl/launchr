// Package builder implements a plugin to build launchr with plugins.
package builder

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/launchrctl/launchr"
)

// ID is a plugin id.
const ID = "builder"

func init() {
	launchr.RegisterPlugin(&Plugin{})
}

// Plugin is a plugin to build launchr application.
type Plugin struct {
}

// PluginInfo implements launchr.Plugin interface.
func (p *Plugin) PluginInfo() launchr.PluginInfo {
	return launchr.PluginInfo{
		ID: ID,
	}
}

// InitApp implements launchr.Plugin interface.
func (p *Plugin) InitApp(*launchr.App) error {
	return nil
}

type builderInput struct {
	name    string
	out     string
	timeout string
	plugins []string
	replace []string
	debug   bool
}

// CobraAddCommands implements launchr.CobraPlugin interface to provide build functionality.
func (p *Plugin) CobraAddCommands(rootCmd *cobra.Command) error {
	// Flag options.
	flags := builderInput{}

	var buildCmd = &cobra.Command{
		Use: "build",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Don't show usage help on a runtime error.
			cmd.SilenceUsage = true
			return Execute(cmd.Context(), &flags)
		},
	}
	// Command flags.
	buildCmd.Flags().StringVarP(&flags.name, "name", "n", "launchr", `Result application name`)
	buildCmd.Flags().StringVarP(&flags.out, "output", "o", "", `Build output file, by default application name is used`)
	buildCmd.Flags().StringVarP(&flags.timeout, "timeout", "t", "120s", `Build timeout duration, example: 0, 100ms, 1h23m`)
	buildCmd.Flags().StringSliceVarP(&flags.plugins, "plugin", "p", nil, `Include PLUGIN into the build with an optional version`)
	buildCmd.Flags().StringSliceVarP(&flags.replace, "replace", "r", nil, `Replace go dependency, see "go mod edit -replace"`)
	buildCmd.Flags().BoolVarP(&flags.debug, "debug", "d", false, `Include debug flags into the build to support go debugging with "delve". If not specified, debugging info is trimmed`)
	rootCmd.AddCommand(buildCmd)
	return nil
}

// Execute runs launchr and executes build of launchr.
func Execute(ctx context.Context, flags *builderInput) error {
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
		LaunchrVersion: launchr.GetVersion(),
		CorePkg:        UsePluginInfo{Path: launchrPkg},
		PkgName:        flags.name,
		ModReplace:     replace,
		Plugins:        plugins,
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
	return builder.Build(ctx)
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
