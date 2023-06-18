package builder

import (
	"context"
	"fmt"
	"strings"

	"github.com/launchrctl/launchr"
)

// Execute runs launchr and executes build of launchr.
func Execute(name string, out string, plugins []string, replace []string, v *launchr.AppVersion, debug bool) error {
	if len(out) == 0 && len(name) > 0 {
		out = "./" + name
	}
	// Collect unique plugins. Include default launchr plugins.
	defaultPlugins := "github.com/launchrctl/launchr/core/plugins"
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
	// Replace module dependencies, e.g. with local paths for development or different version.
	allrepl := map[string]string{}
	for _, rdef := range replace {
		oldnew := strings.SplitN(rdef, "=", 2)
		if len(oldnew) == 1 {
			return fmt.Errorf("incorrect replace definition: %s", rdef)
		}
		allrepl[oldnew[0]] = oldnew[1]
	}

	opts := &BuildOptions{
		LaunchrVersion: v,
		PkgName:        name,
		ModReplace:     allrepl,
		Plugins:        allplugs,
		BuildOutput:    out,
		Debug:          debug,
	}

	builder, err := NewBuilder(opts)
	if err != nil {
		return err
	}
	defer builder.Close()
	ctx := context.Background()
	return builder.Build(ctx)
}
