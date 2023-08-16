package launchr

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"text/template"
)

func (app *appImpl) gen(buildPath string, wordDir string) error {
	var err error
	buildPath, err = filepath.Abs(buildPath)
	if err != nil {
		return err
	}
	wordDir, err = filepath.Abs(wordDir)
	if err != nil {
		return err
	}
	// Clean build path before generating.
	err = filepath.WalkDir(buildPath, func(path string, dir fs.DirEntry, err error) error {
		if path == buildPath {
			return nil
		}
		if dir.IsDir() {
			errRem := os.RemoveAll(path)
			if errRem != nil {
				return errRem
			}
			return filepath.SkipDir
		}
		if dir.Name() != "pkg.go" {
			errRem := os.Remove(path)
			if errRem != nil {
				return errRem
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	// Call generate functions on plugins.
	initSet := make(map[string]struct{})
	for _, p := range getPluginByType[GeneratePlugin](app) {
		genData, err := p.Generate(buildPath, wordDir)
		if err != nil {
			return err
		}
		for _, class := range genData.Plugins {
			initSet[class] = struct{}{}
		}
	}
	if len(initSet) > 0 {
		var tplName = "init.gen"
		tpl, err := template.New(tplName).Parse(initGenTemplate)
		if err != nil {
			return err
		}
		// Generate main.go.
		var buf bytes.Buffer
		err = tpl.Execute(&buf, &initTplVars{
			Plugins: initSet,
		})
		if err != nil {
			return err
		}
		target := filepath.Join(buildPath, tplName+".go")
		err = os.WriteFile(target, buf.Bytes(), 0600)
		if err != nil {
			return err
		}
	}

	return nil
}

// Generate runs generation of included plugins.
func (app *appImpl) Generate() int {
	buildPath := "./gen"
	wd := "./"
	if len(os.Args) > 1 {
		wd = os.Args[1]
	}
	var err error
	if err = app.init(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		return 125
	}
	if err = app.gen(buildPath, wd); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		return 125
	}
	return 0
}

// Gen generates application specific build files and returns os exit code.
func Gen() int {
	return newApp().Generate()
}

type initTplVars struct {
	Plugins map[string]struct{}
}

const initGenTemplate = `
{{- print "// GENERATED. DO NOT EDIT." }}
package gen

import (
	"github.com/launchrctl/launchr"
)

func init() {
	{{- range $class, $ := .Plugins }}
	launchr.RegisterPlugin(&{{$class}}{})
	{{- end }}
}
`
