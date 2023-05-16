package core

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/launchrctl/launchr/core/log"
)

var verbosity = 0
var quiet = false

func setCobraLogger() {
	if quiet {
		// @todo it doesn't really work for cli and docker output, only for logging.
		return
	}
	log.SetGlobalLogger(log.NewPlainLogger(os.Stdout, os.Stderr, nil))
	if verbosity > int(log.ErrLvl) {
		verbosity = int(log.ErrLvl)
	}
	log.SetLevel(log.Level(int(log.ErrLvl) - verbosity))
}

func verbosityFlags(cmd *cobra.Command) {
	// @todo rework to plugins somehow
	cmd.PersistentFlags().CountVarP(&verbosity, "verbose", "v", "log verbosity level, use -vvv DEBUG, -vv WARN, -v INFO")
	cmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "disable stdOut")
}
