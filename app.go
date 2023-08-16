package launchr

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"github.com/spf13/cobra"

	"github.com/launchrctl/launchr/internal/launchr"
	"github.com/launchrctl/launchr/pkg/action"
	"github.com/launchrctl/launchr/pkg/cli"
	_ "github.com/launchrctl/launchr/plugins" // include default plugins
)

// ActionsGroup is a cobra command group definition
var ActionsGroup = &cobra.Group{
	ID:    "actions",
	Title: "Actions:",
}

type appImpl struct {
	rootCmd    *cobra.Command
	streams    cli.Streams
	workDir    string
	cfgDir     string
	services   map[ServiceInfo]Service
	actionMngr action.Manager
	pluginMngr PluginManager
	config     Config
}

// getPluginByType returns specific plugins from the app.
func getPluginByType[T Plugin](app *appImpl) []T {
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

func newApp() *appImpl {
	return &appImpl{}
}

// GetWD implements launchr.App interface.
func (app *appImpl) GetWD() string {
	return app.workDir
}

// Streams implements launchr.App interface.
func (app *appImpl) Streams() cli.Streams {
	return app.streams
}

// AddService implements launchr.App interface.
func (app *appImpl) AddService(s Service) {
	info := s.ServiceInfo()
	launchr.InitServiceInfo(&info, s)
	if _, ok := app.services[info]; ok {
		panic(fmt.Errorf("service %s already exists, review your code", info))
	}
	app.services[info] = s
}

// GetService implements launchr.App interface.
func (app *appImpl) GetService(v interface{}) {
	// Check v is a pointer and implements Service to set a value later.
	t := reflect.TypeOf(v)
	isPtr := t != nil && t.Kind() == reflect.Pointer
	var stype reflect.Type
	if isPtr {
		stype = t.Elem()
	}

	// v must be Service but can't equal it because all elements implement it
	// and the first value will always be returned.
	intService := reflect.TypeOf((*Service)(nil)).Elem()
	if !isPtr || !stype.Implements(intService) || stype == intService {
		panic(fmt.Errorf("argument must be a pointer to a type (interface) implementing Service, %q given", t))
	}
	for _, srv := range app.services {
		st := reflect.TypeOf(srv)
		if st.AssignableTo(stype) {
			reflect.ValueOf(v).Elem().Set(reflect.ValueOf(srv))
			return
		}
	}
	panic(fmt.Sprintf("service %q does not exist", stype))
}

// init initializes application and plugins.
func (app *appImpl) init() error {
	var err error
	// Set working dir and config dir.
	app.cfgDir = "." + name
	app.workDir, err = filepath.Abs("./")
	if err != nil {
		return err
	}
	// Prepare dependencies.
	app.streams = cli.StandardStreams()
	app.services = make(map[ServiceInfo]Service)
	app.actionMngr = action.NewManager()
	app.pluginMngr = launchr.NewPluginManagerWithRegistered()
	app.config = launchr.ConfigFromFS(os.DirFS(app.cfgDir))
	// Register services for other modules.
	app.AddService(app.actionMngr)
	app.AddService(app.pluginMngr)
	app.AddService(app.config)

	// Run OnAppInit hook.
	for _, p := range getPluginByType[OnAppInitPlugin](app) {
		if err = p.OnAppInit(app); err != nil {
			return err
		}
	}

	return nil
}

func (app *appImpl) exec() error {
	// Set root cobra command.
	var rootCmd = &cobra.Command{
		Use: name,
		//Short: "", // @todo
		//Long:  ``, // @todo
		SilenceErrors: true, // Handled manually.
		Version:       version,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	// Quick parse arguments to see if a version was requested.
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		if args[i] == "--version" {
			rootCmd.SetVersionTemplate(Version().Full())
			break
		}
	}

	// Convert actions to cobra commands.
	actions := app.actionMngr.All()
	if len(actions) > 0 {
		rootCmd.AddGroup(ActionsGroup)
	}
	for _, cmdDef := range actions {
		cobraCmd, err := action.CobraImpl(cmdDef, app.Streams(), app.config, ActionsGroup)
		if err != nil {
			return err
		}
		rootCmd.AddCommand(cobraCmd)
	}

	// Add cobra commands from plugins.
	for _, p := range getPluginByType[CobraPlugin](app) {
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
func (app *appImpl) Execute() int {
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
	return newApp().Execute()
}
