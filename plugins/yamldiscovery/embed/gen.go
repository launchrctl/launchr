// Package embed provides yaml discovery with embed actions definition.
package embed

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/launchrctl/launchr/core"
	"github.com/launchrctl/launchr/plugins/yamldiscovery"
)

const pluginTemplate = `
{{- print "// GENERATED. DO NOT EDIT." }}
package main

import (
	_ "embed"

	launchr "github.com/launchrctl/launchr/core"
	"github.com/launchrctl/launchr/plugins/yamldiscovery"
)

//go:embed {{.ActionsTarPath}}
var tarFsBytes []byte

type {{.StructName}} struct {
	app *launchr.App
}

// PluginInfo implements launchr.Plugin interface.
func (p *{{.StructName}}) PluginInfo() launchr.PluginInfo {
	return launchr.PluginInfo{
		ID: "{{.ID}}",
	}
}

// InitApp implements launchr.Plugin interface.
func (p *{{.StructName}}) InitApp(app *launchr.App) error {
	fs, err := yamldiscovery.UntarFsBytes(tarFsBytes)
	if err == nil {
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

// Generate implements core.GeneratePlugin interface.
func (p *Plugin) Generate(buildPath string) (*core.PluginGeneratedData, error) {
	dp, err := yamldiscovery.GetDiscoveryPath()
	if err != nil {
		return nil, err
	}
	// Generate actions tar.
	fmt.Println("[INFO] Discovering actions")
	tarName, cmds, err := createActionTar(os.DirFS(dp), buildPath)
	if err != nil {
		return nil, err
	}
	fmt.Println("[INFO] Discovered:")
	for i, cmd := range cmds {
		fmt.Printf("%d. %s\n", i+1, cmd.CommandName)
	}
	var id = yamldiscovery.ID + ".gen"
	tpl, err := template.New(id).Parse(pluginTemplate)
	if err != nil {
		return nil, err
	}

	fmt.Println("[INFO] Generating embed actions go file")
	var buf bytes.Buffer
	structName := core.ToCamelCase(id, false)
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

	return &core.PluginGeneratedData{
		Plugins: []string{structName},
	}, nil
}
