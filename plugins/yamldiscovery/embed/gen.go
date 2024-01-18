// Package embed provides yaml discovery with embed actions definition.
package embed

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/launchrctl/launchr/internal/launchr"
)

const pluginTemplate = `
{{- print "// GENERATED. DO NOT EDIT." }}
package gen

import (
	_ "embed"

	"github.com/launchrctl/launchr"
	yamlembed "github.com/launchrctl/launchr/pkg/plugins/yamldiscovery/embed"
)

//go:embed {{.ActionsTarPath}}
var tarFsBytes []byte

type {{.StructName}} struct {
	app launchr.App
}

// PluginInfo implements launchr.Plugin interface.
func (p *{{.StructName}}) PluginInfo() launchr.PluginInfo {
	return launchr.PluginInfo{}
}

// OnAppInit implements launchr.Plugin interface.
func (p *{{.StructName}}) OnAppInit(app launchr.App) error {
	fs, err := yamlembed.UntarFsBytes(tarFsBytes)
	if err == nil {
        // @fixme
		app.SetFS(fs)
	}
	return err
}
`

type pluginVars struct {
	ID             string
	ActionsTarPath string
	StructName     string
}

// Generate implements launchr.GeneratePlugin interface.
func (p *Plugin) Generate(buildPath string, workDir string) (*launchr.PluginGeneratedData, error) {
	// Generate actions tar.
	fmt.Println("[INFO] Discovering actions")
	tarName, actions, err := createActionTar(os.DirFS(workDir), buildPath)
	if err != nil {
		return nil, err
	}
	fmt.Println("[INFO] Discovered:")
	for i, a := range actions {
		fmt.Printf("%d. %s\n", i+1, a.ID)
	}
	var id = "actions.yamldiscovery.gen"
	tpl, err := template.New(id).Parse(pluginTemplate)
	if err != nil {
		return nil, err
	}

	fmt.Println("[INFO] Generating embed actions go file")
	var buf bytes.Buffer
	structName := "ActionsYamlDiscoveryGen"
	err = tpl.Execute(&buf, &pluginVars{
		ID:             id,
		StructName:     structName,
		ActionsTarPath: tarName,
	})
	if err != nil {
		return nil, err
	}
	target := filepath.Join(buildPath, id+".go")
	err = os.WriteFile(target, buf.Bytes(), 0600)
	if err != nil {
		return nil, err
	}

	return &launchr.PluginGeneratedData{
		Plugins: []string{structName},
	}, nil
}
