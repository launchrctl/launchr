// Package launchr has application implementation.
package launchr

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/launchrctl/launchr/internal/launchr/config"
	"github.com/launchrctl/launchr/pkg/action"
	"github.com/launchrctl/launchr/pkg/cli"
)

// ActionsGroup is a cobra command group definition
var ActionsGroup = &cobra.Group{
	ID:    "actions",
	Title: "Actions:",
}

// App holds app related global variables.
type App struct {
	rootCmd    *cobra.Command
	streams    cli.Streams
	workDir    string
	cfgDir     string
	version    *AppVersion
	services   map[ServiceInfo]Service
	actionMngr action.Manager
	pluginMngr PluginManager
	globalCfg  GlobalConfig
}

// NewApp constructs app implementation.
func NewApp() *App {
	return &App{}
}

// GetWD provides app's working dir.
func (app *App) GetWD() string {
	return app.workDir
}

// Streams returns application cli.
func (app *App) Streams() cli.Streams {
	return app.streams
}

// AddService registers a service in the app.
func (app *App) AddService(s Service) {
	info := s.ServiceInfo()
	if _, ok := app.services[info]; ok {
		panic(fmt.Errorf("service %s already exists, review your code", info.ID))
	}
	app.services[info] = s
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
	app.streams = cli.StandardStreams()
	app.services = make(map[ServiceInfo]Service)
	app.actionMngr = action.NewManager()
	app.pluginMngr = pluginManagerMap(registeredPlugins)
	app.globalCfg = config.GlobalConfigFromFS(os.DirFS(app.cfgDir))
	app.AddService(app.actionMngr)
	app.AddService(app.pluginMngr)
	app.AddService(app.globalCfg)

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
		cobraCmd, err := action.CobraImpl(cmdDef, app.Streams(), app.globalCfg, ActionsGroup)
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
	rootCmd.SetIn(app.streams.In())
	rootCmd.SetOut(app.streams.Out())
	rootCmd.SetErr(app.streams.Err())
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
