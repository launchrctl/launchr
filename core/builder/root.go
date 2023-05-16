package builder

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/launchrctl/launchr/core"
)

type strSlice []string

func (i *strSlice) String() string {
	return ""
}

func (i *strSlice) Set(value string) error {
	*i = append(*i, value)
	return nil
}

// Flag options.
var (
	version bool
	debug   bool
	out     string
	replace strSlice
	plugins strSlice
)

const usage = `Usage:
    launchr [--plugin MODULE[@v1.1]]... [--replace OLD=NEW]... [-o OUTPUT] [--debug]
    launchr --version
    launchr --help
Options:
    -o, --output OUTPUT         Build output file, by default "./launchr".
    -p, --plugin PLUGIN[@v1.1]  Include PLUGIN into the build with an optional version.
    -r, --replace OLD=NEW       Replace go dependency, see "go mod edit -replace".
    -d, --debug                 Include debug flags into the build to support go debugging with "delve".
								If not specified, debugging info is trimmed.
    -v, --version               Output launchr version.
    -h, --help                  Output launchr usage help message.
`

// Execute runs launchr and executes build of launchr.
func Execute(v *core.AppVersion) error {
	flag.BoolVar(&version, "v", false, "")
	flag.BoolVar(&version, "version", false, "")
	flag.BoolVar(&debug, "d", false, "")
	flag.BoolVar(&debug, "debug", false, "")
	flag.Var(&replace, "r", "")
	flag.Var(&replace, "replace", "")
	flag.Var(&plugins, "p", "")
	flag.Var(&plugins, "plugin", "")
	flag.StringVar(&out, "o", "./launchr", "")
	flag.StringVar(&out, "output", "./launchr", "")
	flag.Usage = func() { fmt.Print(usage) }
	flag.Parse()

	// Return early version.
	if version {
		fmt.Println(v)
		return nil
	}

	// Collect unique plugins.
	// @fixme do not hardcode
	yamlplugin := "github.com/launchrctl/launchr/plugins/yamldiscovery"
	setplugs := map[string]UsePluginInfo{
		yamlplugin: {yamlplugin, ""},
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
	// Collect replace definitions.
	localPath, _ := os.Getwd() // @fixme do not hardcode
	allrepl := map[string]string{
		// Replace private repo with a local path.
		"github.com/launchrctl/launchr": localPath,
	}
	for _, rdef := range replace {
		oldnew := strings.SplitN(rdef, "=", 2)
		if len(oldnew) == 1 {
			return fmt.Errorf("incorrect replace definition: %s", rdef)
		}
		allrepl[oldnew[0]] = oldnew[1]
	}

	opts := &BuildOptions{
		LaunchrVersion: v,
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
