// Package launchr has application implementation.
package launchr

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/launchrctl/launchr/pkg/cli"
	"github.com/launchrctl/launchr/pkg/log"
)

// App holds app related global variables.
type App struct {
	cmd     *cobra.Command
	cli     cli.Cli
	workDir string
	cfgDir  string
	version *AppVersion
	plugins map[PluginInfo]Plugin
}

// NewApp constructs app implementation.
func NewApp() *App {
	return &App{}
}

// GetWD provides app's working dir.
func (app *App) GetWD() string {
	return app.workDir
}

// GetCli returns application cli.
func (app *App) GetCli() cli.Cli {
	return app.cli
}

// SetCli sets application cli.
func (app *App) SetCli(c cli.Cli) {
	app.cli = c
}

// Plugins returns installed app plugins.
func (app *App) Plugins() map[PluginInfo]Plugin {
	return app.plugins
}

// Init initializes application and plugins.
func (app *App) Init() error {
	var err error
	app.version = GetVersion()
	// Global configuration.
	app.cfgDir = fmt.Sprintf(".%s", app.version.Name)
	app.workDir, err = filepath.Abs("./")
	if err != nil {
		return err
	}
	appCli, err := cli.NewAppCli(
		cli.WithStandardStreams(),
		cli.WithGlobalConfigFromDir(os.DirFS(app.cfgDir)),
	)
	if err != nil {
		return err
	}
	app.SetCli(appCli)

	// Initialize the plugins.
	app.plugins = registeredPlugins
	for _, p := range app.plugins {
		if err = p.InitApp(app); err != nil {
			return err
		}
	}

	return nil
}

func (app *App) exec() error {
	// Set root cobra command.
	var rootCmd = &cobra.Command{
		Use: Name,
		//Short: "", // @todo
		//Long:  ``, // @todo
		Version: Version,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			setCobraLogger()
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	rootCmd.SetVersionTemplate(app.version.String())
	verbosityFlags(rootCmd)
	for _, p := range app.Plugins() {
		p, ok := p.(CobraPlugin)
		if ok {
			if err := p.CobraAddCommands(rootCmd); err != nil {
				return err
			}
		}
	}

	// Set streams.
	app.cmd = rootCmd
	rootCmd.SetIn(app.cli.In())
	rootCmd.SetOut(app.cli.Out())
	rootCmd.SetErr(app.cli.Err())
	return app.cmd.Execute()
}

// Execute is a cobra entrypoint to the launchr app.
func (app *App) Execute() int {
	if err := app.exec(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

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

// Run executes launchr application and returns os exit code.
func Run() int {
	app := NewApp()
	if err := app.Init(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 125
	}
	return app.Execute()
}
