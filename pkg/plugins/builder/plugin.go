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

// CobraAddCommands implements launchr.CobraPlugin interface to provide build functionality.
func (p *Plugin) CobraAddCommands(rootCmd *cobra.Command) error {
	// Flag options.
	var (
		name    string
		out     string
		timeout int
		plugins []string
		replace []string
		debug   bool
	)

	var buildCmd = &cobra.Command{
		Use: "build",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Don't show usage help on a runtime error.
			cmd.SilenceUsage = true
			ctx := cmd.Context()
			if timeout != 0 {
				// Execute build.
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, time.Second*time.Duration(timeout))
				defer cancel()
			}
			allplugs, err := parsePlugins(plugins)
			if err != nil {
				return err
			}
			allrepl, err := parseReplace(replace)
			if err != nil {
				return err
			}

			if len(out) == 0 && len(name) > 0 {
				out = "./" + name
			}

			opts := &BuildOptions{
				LaunchrVersion: launchr.GetVersion(),
				CorePkg:        UsePluginInfo{Package: launchrPkg},
				PkgName:        name,
				ModReplace:     allrepl,
				Plugins:        allplugs,
				BuildOutput:    out,
				Debug:          debug,
			}

			return Execute(ctx, opts)
		},
	}
	buildCmd.Flags().StringVarP(&name, "name", "n", "launchr", `Result application name`)
	buildCmd.Flags().StringVarP(&out, "output", "o", "", `Build output file, by default application name is used`)
	buildCmd.Flags().IntVarP(&timeout, "timeout", "t", 120, `Build timeout in seconds`)
	buildCmd.Flags().StringSliceVarP(&plugins, "plugin", "p", nil, `Include PLUGIN into the build with an optional version`)
	buildCmd.Flags().StringSliceVarP(&replace, "replace", "r", nil, `Replace go dependency, see "go mod edit -replace"`)
	buildCmd.Flags().BoolVarP(&debug, "debug", "d", false, `Include debug flags into the build to support go debugging with "delve". If not specified, debugging info is trimmed`)
	rootCmd.AddCommand(buildCmd)
	return nil
}

// Execute runs launchr and executes build of launchr.
func Execute(ctx context.Context, opts *BuildOptions) error {
	builder, err := NewBuilder(opts)
	if err != nil {
		return err
	}
	defer builder.Close()
	return builder.Build(ctx)
}

func parsePlugins(plugins []string) ([]UsePluginInfo, error) {
	// Collect unique plugins. Include default launchr plugins.
	defaultPlugins := launchrPkg + "/pkg/plugins"
	setplugs := map[string]UsePluginInfo{
		defaultPlugins: {defaultPlugins, ""},
	}
	for _, pdef := range plugins {
		pv := strings.SplitN(pdef, "@", 2)
		if len(pv) == 1 {
			pv = append(pv, "")
		}
		setplugs[pv[0]] = UsePluginInfo{pv[0], pv[1]}
	}
	allplugs := make([]UsePluginInfo, 0, len(setplugs))
	for _, p := range setplugs {
		allplugs = append(allplugs, p)
	}
	return allplugs, nil
}

func parseReplace(replace []string) (map[string]string, error) {
	// Replace module dependencies, e.g. with local paths for development or different version.
	allrepl := map[string]string{}
	for _, rdef := range replace {
		oldnew := strings.SplitN(rdef, "=", 2)
		if len(oldnew) == 1 {
			return nil, fmt.Errorf("incorrect replace definition: %s", rdef)
		}
		allrepl[oldnew[0]] = oldnew[1]
	}
	return allrepl, nil
}
