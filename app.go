// Package launchr has application implementation.
package launchr

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

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
	rootCmd     *cobra.Command
	cli         cli.Cli
	workDir     string
	cfgDir      string
	version     *AppVersion
	actionMngr  ActionManager
	serviceMngr ServiceManager
	pluginMngr  PluginManager
}

// NewApp constructs app implementation.
func NewApp() *App {
	return &App{}
}

// GetWD provides app's working dir.
func (app *App) GetWD() string {
	return app.workDir
}

// GetCfgDir returns config directory path.
func (app *App) GetCfgDir() string {
	return app.cfgDir
}

// GetCli returns application cli.
func (app *App) GetCli() cli.Cli {
	return app.cli
}

// SetCli sets application cli.
func (app *App) SetCli(c cli.Cli) {
	app.cli = c
}

// ServiceManager returns application service manager.
func (app *App) ServiceManager() ServiceManager {
	return app.serviceMngr
}

// init initializes application and plugins.
func (app *App) init() error {
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
	app.serviceMngr = newServiceManager()
	app.actionMngr = newActionManager()
	app.pluginMngr = pluginManagerMap(registeredPlugins)
	app.serviceMngr.Add(ServiceManagerID, app.serviceMngr)
	app.serviceMngr.Add(ActionManagerID, app.actionMngr)
	app.serviceMngr.Add(PluginManagerID, app.pluginMngr)

	// Initialize the plugins.
	for _, p := range app.pluginMngr.All() {
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
		SilenceErrors: true, // Handled manually.
		Version:       Version,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	rootCmd.SetVersionTemplate(app.version.String())

	// Convert actions to cobra commands.
	actions := app.actionMngr.All()
	if len(actions) > 0 {
		rootCmd.AddGroup(ActionsGroup)
	}
	for _, cmdDef := range actions {
		cobraCmd, err := cobraadapter.GetActionImpl(app.GetCli(), cmdDef, ActionsGroup)
		if err != nil {
			return err
		}
		rootCmd.AddCommand(cobraCmd)
	}

	// Add cobra commands from plugins.
	for _, p := range GetPluginByType[CobraPlugin](app) {
		if err := p.CobraAddCommands(rootCmd); err != nil {
			return err
		}
	}

	// Set io streams.
	app.rootCmd = rootCmd
	rootCmd.SetIn(app.cli.In())
	rootCmd.SetOut(app.cli.Out())
	rootCmd.SetErr(app.cli.Err())
	return app.rootCmd.Execute()
}

// Execute is a cobra entrypoint to the launchr app.
func (app *App) Execute() int {
	var err error
	if err = app.init(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		return 125
	}
	if err = app.exec(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		return 1
	}
	return 0
}

// Run executes launchr application.
func Run() int {
	return NewApp().Execute()
}
