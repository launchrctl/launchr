// Package builder implements a plugin to build launchr with plugins.
package builder

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"time"

	"github.com/launchrctl/launchr/internal/launchr"
	"github.com/launchrctl/launchr/pkg/action"
)

//go:embed action.yaml
var actionYaml []byte

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
	action.WithLogger
	action.WithTerm

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
	actionYaml = bytes.Replace(actionYaml, []byte("DEFAULT_NAME_PLACEHOLDER"), []byte(p.app.Name()), 1)
	return nil
}

// DiscoverActions implements [launchr.ActionDiscoveryPlugin] interface.
func (p *Plugin) DiscoverActions(_ context.Context) ([]*action.Action, error) {
	a := action.NewFromYAML("build", actionYaml)
	a.SetRuntime(action.NewFnRuntime(func(ctx context.Context, a *action.Action) error {
		input := a.Input()
		flags := builderInput{
			name:    input.Opt("name").(string),
			out:     input.Opt("output").(string),
			version: input.Opt("build-version").(string),
			timeout: input.Opt("timeout").(string),
			tags:    action.InputOptSlice[string](input, "tag"),
			plugins: action.InputOptSlice[string](input, "plugin"),
			replace: action.InputOptSlice[string](input, "replace"),
			debug:   input.Opt("debug").(bool),
			nocache: input.Opt("no-cache").(bool),
		}

		log := launchr.Log()
		if rt, ok := a.Runtime().(action.RuntimeLoggerAware); ok {
			log = rt.LogWith()
		}
		flags.SetLogger(log)

		term := launchr.Term()
		if rt, ok := a.Runtime().(action.RuntimeTermAware); ok {
			term = rt.Term()
		}
		flags.SetTerm(term)

		return Execute(ctx, p.app.Streams(), &flags)
	}))
	return []*action.Action{a}, nil
}

// Generate implements [launchr.GeneratePlugin] interface.
func (p *Plugin) Generate(config launchr.GenerateConfig) error {
	launchr.Term().Info().Println("Generating main.go file")
	tpl := launchr.Template{
		Tmpl: tmplMain,
		Data: &buildVars{
			CorePkg: corePkgInfo(),
		},
	}
	return tpl.WriteFile(filepath.Join(config.BuildDir, "main.go"))
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
	builder.WithLogger = flags.WithLogger
	builder.WithTerm = flags.WithTerm

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
