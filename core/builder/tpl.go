package builder

import (
	"text/template"

	"github.com/launchrctl/launchr/core"
)

const mainTemplate = `
{{- if .BuildTags}}{{print "//go:build " .BuildTags}}{{end}}
{{print "// Built by Launchr"}}
package main

import (
	"bytes"
	"fmt"
	"os"

	launchr "github.com/launchrctl/launchr/core"

    // Plugins
	{{- range .Plugins}}
	_ "{{.}}"
	{{- end}}
)

{{define "version"}}
{{- with .BuildVersion -}}
  {{.Name}} version {{.Version}}{{if ne .GitHash ""}} ({{.GitHash}}){{end}} {{print .OS "/" .Arch .Arm}}
{{- end}}
Built with {{with .LaunchrVersion -}}
  {{.Name}} ({{.Version}}{{if ne .GitHash ""}} {{.GitHash}}{{end}}) {{print .OS "/" .Arch .Arm}}
{{- end -}}
{{with .BuildVersion}} and {{.GoVersion}} at {{.BuildDate}}
{{- end}}

Plugins:
  {{- range .Plugins}}
  - {{.}}
  {{- end}}
{{end}}

{{- /* inline version in a multiline string */ -}}
var version = ` + "`{{template \"version\" .}}`" + `

func main() {
	app := launchr.NewApp()
	app.SetVersion(bytes.NewBufferString(version))
	if err := app.Init(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(125)
	}
	os.Exit({{if .ExecFn}}{{.ExecFn}}{{else}}app.Execute(){{end}})
}
`

type buildVars struct {
	BuildTags      string
	LaunchrVersion *core.AppVersion
	BuildVersion   *core.AppVersion
	Plugins        []UsePluginInfo
	ExecFn         string
}

func newAppTpl() (*template.Template, error) {
	return template.
		New("main").
		Parse(mainTemplate)
}
