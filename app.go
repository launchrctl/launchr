// Package launchr has application implementation.
package launchr

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/launchrctl/launchr/pkg/action"
	"github.com/launchrctl/launchr/pkg/cli"
	"github.com/launchrctl/launchr/pkg/cobraadapter"
)

// ActionsGroup is a cobra command group definition
var ActionsGroup = &cobra.Group{
	ID:    "actions",
	Title: "Actions:",
}

// App holds app related global variables.
type App struct {
	cmd     *cobra.Command
	cli     cli.Cli
	workDir string
	cfgDir  string
	version *AppVersion
	plugins map[PluginInfo]Plugin
	actions map[string]*action.Command
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

// AddActionCommand adds an action to the app.
func (app *App) AddActionCommand(cmd *action.Command) {
	app.actions[cmd.CommandName] = cmd
}

func (app *App) GetActionCommands() map[string]*action.Command {
	return app.actions
}

// Init initializes application and plugins.
func (app *App) Init() error {
	var err error
	app.version = GetVersion()
	// Global configuration.
	app.cfgDir = fmt.Sprintf(".%s", app.version.Name)
	app.workDir, err = filepath.Abs("./")
	app.actions = make(map[string]*action.Command)
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
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	rootCmd.SetVersionTemplate(app.version.String())

	// Convert actions to cobra commands.
	if len(app.actions) > 0 {
		rootCmd.AddGroup(ActionsGroup)
	}
	for _, cmdDef := range app.actions {
		cobraCmd, err := cobraadapter.GetActionImpl(app.GetCli(), cmdDef, ActionsGroup)
		if err != nil {
			return err
		}
		rootCmd.AddCommand(cobraCmd)
	}

	// Add cobra commands from plugins.
	for _, p := range app.Plugins() {
		p, ok := p.(CobraPlugin)
		if ok {
			if err := p.CobraAddCommands(rootCmd); err != nil {
				return err
			}
		}
	}

	// Set io streams.
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

// Run executes launchr application and returns os exit code.
func Run() int {
	app := NewApp()
	if err := app.Init(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 125
	}
	return app.Execute()
}
