package genaction

import (
	"context"
	"os"
	"path/filepath"

	"github.com/launchrctl/launchr"
	"github.com/launchrctl/launchr/pkg/action"
)

// pluginTemplate is a go file that will be generated and included in the build.
const pluginTemplate = `package main
import (
	_ "embed"
	my "{{.Pkg}}"
)
//go:embed {{.Yaml}}
var y []byte
func init() { my.ActionYaml = y }`

// ActionYaml is a yaml content that will be set from an embedded file in [pluginTemplate].
var ActionYaml []byte

func init() {
	launchr.RegisterPlugin(&Plugin{})
}

// Plugin is a test plugin declaration.
type Plugin struct{}

// PluginInfo implements [launchr.Plugin] interface.
func (p *Plugin) PluginInfo() launchr.PluginInfo {
	return launchr.PluginInfo{}
}

// Generate implements [launchr.GeneratePlugin] interface.
func (p *Plugin) Generate(config launchr.GenerateConfig) error {
	launchr.Term().Info().Printfln("Generating genaction...")

	actionyaml := "action.yaml"
	const yaml = "{ runtime: plugin, action: { title: My plugin } }"
	type tplvars struct {
		Pkg  string
		Yaml string
	}

	tpl := launchr.Template{Tmpl: pluginTemplate, Data: tplvars{
		Pkg:  "example.com/genaction",
		Yaml: actionyaml,
	}}
	err := tpl.WriteFile(filepath.Join(config.BuildDir, "genaction.gen.go"))
	if err != nil {
		return err
	}

	err = os.WriteFile(filepath.Join(config.BuildDir, actionyaml), []byte(yaml), 0600)
	if err != nil {
		return err
	}

	return nil
}

// DiscoverActions implements [launchr.ActionDiscoveryPlugin] interface.
func (p *Plugin) DiscoverActions(_ context.Context) ([]*action.Action, error) {
	a := action.NewFromYAML("genaction:example", ActionYaml)
	a.SetRuntime(action.NewFnRuntime(func(_ context.Context, a *action.Action) error {
		launchr.Term().Println(helloWorldStr())
		return nil
	}))

	return []*action.Action{a}, nil
}
