package launchr

import (
	"bytes"
	"fmt"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"text/template"
	_ "unsafe" // Use unsafe to have linked variables from the main package.
)

// Link variables to the main package to get the values from ldflags.
var (
	//go:linkname name github.com/launchrctl/launchr.name
	name string
	//go:linkname version github.com/launchrctl/launchr.version
	version string
	//go:linkname builtWith github.com/launchrctl/launchr.builtWith
	builtWith  string
	appVersion *AppVersion
)

// Version provides app version info.
func Version() *AppVersion {
	if appVersion == nil {
		appVersion = NewVersion(name, version, builtWith, registeredPlugins)
	}
	return appVersion
}

// NewVersion creates version info with used plugins info.
func NewVersion(name, ver, bwith string, plugins PluginsMap) *AppVersion {
	buildInfo, _ := debug.ReadBuildInfo()
	// Add self as a dependency to get version for it also.
	buildInfo.Deps = append(buildInfo.Deps, &buildInfo.Main)
	// Check core version when built or used in a plugin.
	var coreRep string
	coreVer, coreRep := getCoreInfo(ver, buildInfo)
	if bwith == "" {
		ver = coreVer
	}

	return &AppVersion{
		Name:        name,
		Version:     ver,
		OS:          runtime.GOOS,
		Arch:        runtime.GOARCH,
		CoreVersion: coreVer,
		CoreReplace: coreRep,
		BuiltWith:   bwith,
		Plugins:     getPluginModules(plugins, buildInfo),
	}
}

// String implements Stringer interface.
func (v *AppVersion) String() string {
	return v.Full()
}

// Full outputs version string in a full format.
func (v *AppVersion) Full() string {
	b := &bytes.Buffer{}
	err := versionTmpl.Execute(b, v)
	if err != nil {
		panic(err)
	}
	return b.String()
}

// Short outputs a short version string.
func (v *AppVersion) Short() string {
	return fmt.Sprintf("%s version %s %s/%s", v.Name, v.Version, v.OS, v.Arch)
}

func getCoreInfo(v string, bi *debug.BuildInfo) (ver string, repl string) {
	ver = v
	// Return for self, it's always (devel).
	if bi == nil || bi.Main.Path == PkgPath {
		return
	}
	// Get version if built by builder.
	for _, d := range bi.Deps {
		if d.Path == PkgPath {
			ver = d.Version
			if d.Replace != nil {
				repl = fmt.Sprintf("%s %s => %s %s", d.Path, d.Version, d.Replace.Path, d.Replace.Version)
			}
			return
		}
	}
	return
}

func getPluginModules(plugins PluginsMap, bi *debug.BuildInfo) []string {
	if bi == nil {
		return nil
	}

	res := make([]string, 0, len(plugins))
	for pi := range plugins {
		if strings.HasPrefix(pi.pkgPath, PkgPath) {
			// Do not include info about the default package.
			continue
		}
		for _, d := range bi.Deps {
			// Path may be empty on "go run".
			if d.Path != "" && strings.HasPrefix(pi.pkgPath, d.Path) {
				s := fmt.Sprintf("%s %s", pi.pkgPath, d.Version)
				if d.Replace != nil {
					s = fmt.Sprintf("%s => %s %s", s, d.Replace.Path, d.Replace.Version)
				}
				res = append(res, s)
			}
		}
	}
	sort.Strings(res)
	return res
}

var versionTmpl = template.Must(template.New("version").Parse(versionTmplStr))

const versionTmplStr = `
{{- .Short}}
{{- if .BuiltWith}}
Built with {{.BuiltWith}}
{{- end}}
{{- if ne .CoreVersion .Version}}
Core version: {{.CoreVersion}}
{{- end}}
{{- if .CoreReplace}}
Core replace: {{.CoreReplace}}
{{- end}}
{{- if .Plugins}}
Plugins:
  {{- range .Plugins}}
  - {{.}}
  {{- end}}
{{end}}`
