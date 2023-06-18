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

// PluginInfo provides information about the plugin.
type PluginInfo struct {
	ID     string
	Weight int
}

// Plugin is a common interface for launchr plugins.
type Plugin interface {
	PluginInfo() PluginInfo
	InitApp(app *App) error
}

// CobraPlugin is an interface to implement a plugin for cobra.
type CobraPlugin interface {
	Plugin
	CobraAddCommands(*cobra.Command) error
}

// PluginGeneratedData is a struct containing a result information of plugin generation.
type PluginGeneratedData struct {
	Plugins []string
}

// GeneratePlugin is an interface to generate supporting files before build.
type GeneratePlugin interface {
	Plugin
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
