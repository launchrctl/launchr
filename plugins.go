package launchr

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var registeredPlugins = make(map[PluginInfo]Plugin)

// RegisterPlugin add a plugin to global pull.
func RegisterPlugin(p Plugin) {
	info := p.PluginInfo()
	if _, ok := registeredPlugins[info]; ok {
		panic(fmt.Errorf("plugin %s already exists, review the build", info.ID))
	}
	registeredPlugins[info] = p
}

// GetPluginByType returns specific plugins from the app.
func GetPluginByType[T Plugin](app *App) []T {
	plugins := app.pluginMngr.All()
	res := make([]T, 0, len(plugins))
	for _, p := range plugins {
		p, ok := p.(T)
		if ok {
			res = append(res, p)
		}
	}
	return res
}

// PluginInfo provides information about the plugin and is used as a unique data to indentify a plugin.
type PluginInfo struct {
	ID string
}

// Plugin is a common interface for launchr plugins.
type Plugin interface {
	// PluginInfo requests a type to provide information about the plugin.
	// The Plugin info is used as a unique data to indentify a plugin.
	PluginInfo() PluginInfo
	// InitApp is hook function called on application initialisation.
	// Plugins may save app global object, retrieve or provide services here.
	InitApp(app *App) error
}

// CobraPlugin is an interface to implement a plugin for cobra.
type CobraPlugin interface {
	Plugin
	// CobraAddCommands is a hook called when cobra root command is available.
	// Plugins may register its command line commands here.
	CobraAddCommands(*cobra.Command) error
}

// PluginGeneratedData is a struct containing a result information of plugin generation.
type PluginGeneratedData struct {
	Plugins []string
}

// GeneratePlugin is an interface to generate supporting files before build.
type GeneratePlugin interface {
	Plugin
	// Generate is a function called when application is generating code and assets for the build.
	Generate(buildPath string, workDir string) (*PluginGeneratedData, error)
}

// ToCamelCase converts a string to CamelCase
func ToCamelCase(s string, capFirst bool) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}

	n := strings.Builder{}
	n.Grow(len(s))
	capNext := capFirst
	for i, v := range []byte(s) {
		vIsCap := v >= 'A' && v <= 'Z'
		vIsLow := v >= 'a' && v <= 'z'
		if capNext {
			if vIsLow {
				v += 'A'
				v -= 'a'
			}
		} else if i == 0 {
			if vIsCap {
				v += 'a'
				v -= 'A'
			}
		}
		if vIsCap || vIsLow {
			n.WriteByte(v)
			capNext = false
		} else if vIsNum := v >= '0' && v <= '9'; vIsNum {
			n.WriteByte(v)
			capNext = true
		} else {
			capNext = v == '_' || v == ' ' || v == '-' || v == '.'
		}
	}
	return n.String()
}
