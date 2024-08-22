// Package verbosity is a plugin of launchr to configure log level of the app.
package verbosity

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/launchrctl/launchr/internal/launchr"
	"github.com/launchrctl/launchr/pkg/log"
)

func init() {
	launchr.RegisterPlugin(&Plugin{})
}

// Plugin is launchr plugin to set verbosity of the application.
type Plugin struct{}

// PluginInfo implements launchr.Plugin interface.
func (p Plugin) PluginInfo() launchr.PluginInfo {
	return launchr.PluginInfo{}
}

// CobraAddCommands implements launchr.CobraPlugin interface to set app verbosity.
func (p Plugin) CobraAddCommands(rootCmd *cobra.Command) error {
	verbosity := 0
	quiet := false
	rootCmd.PersistentFlags().CountVarP(&verbosity, "verbose", "v", "log verbosity level, use -vvv DEBUG, -vv WARN, -v INFO")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "disable stdOut")
	rootCmd.PersistentPreRunE = func(_ *cobra.Command, _ []string) error {
		// @todo logger is not set on preflight like "--help", but needed if fails to boot for debugging.
		if quiet {
			// @todo it doesn't really work for cli and docker output, only for logging.
			return nil
		}
		log.SetGlobalLogger(log.NewPlainLogger(os.Stdout, os.Stderr, nil))
		if verbosity > int(log.ErrLvl) {
			verbosity = int(log.ErrLvl)
		}
		log.SetLevel(log.Level(int(log.ErrLvl) - verbosity))
		return nil
	}
	return nil
}
