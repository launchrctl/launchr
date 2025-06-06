package launchr

import (
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"

	"github.com/launchrctl/launchr/internal/launchr"
	"github.com/launchrctl/launchr/pkg/action"
	_ "github.com/launchrctl/launchr/plugins" // include default plugins
)

type appImpl struct {
	// Cli related.
	cmd      *Command
	earlyCmd launchr.CmdEarlyParsed

	// FS related.
	mFS     []ManagedFS
	workDir string
	cfgDir  string

	// Services.
	streams    Streams
	services   map[ServiceInfo]Service
	pluginMngr PluginManager
}

func newApp() *appImpl {
	return &appImpl{}
}

func (app *appImpl) Name() string         { return name }
func (app *appImpl) GetWD() string        { return app.workDir }
func (app *appImpl) Streams() Streams     { return app.streams }
func (app *appImpl) SetStreams(s Streams) { app.streams = s }

func (app *appImpl) RegisterFS(fs ManagedFS)      { app.mFS = append(app.mFS, fs) }
func (app *appImpl) GetRegisteredFS() []ManagedFS { return app.mFS }

func (app *appImpl) SensitiveWriter(w io.Writer) io.Writer {
	return NewMaskingWriter(w, app.SensitiveMask())
}
func (app *appImpl) SensitiveMask() *SensitiveMask { return launchr.GlobalSensitiveMask() }

func (app *appImpl) RootCmd() *Command                      { return app.cmd }
func (app *appImpl) CmdEarlyParsed() launchr.CmdEarlyParsed { return app.earlyCmd }

func (app *appImpl) AddService(s Service) {
	info := s.ServiceInfo()
	launchr.InitServiceInfo(&info, s)
	if _, ok := app.services[info]; ok {
		panic(fmt.Errorf("service %s already exists, review your code", info))
	}
	app.services[info] = s
}

func (app *appImpl) GetService(v any) {
	// Check v is a pointer and implements [Service] to set a value later.
	t := reflect.TypeOf(v)
	isPtr := t != nil && t.Kind() == reflect.Pointer
	var stype reflect.Type
	if isPtr {
		stype = t.Elem()
	}

	// v must be [Service] but can't equal it because all elements implement it
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
	// Set root command.
	app.cmd = &Command{
		Use:           name,
		Short:         name + ` is a versatile CLI action runner that executes tasks defined in local or embeded yaml files across multiple runtimes`,
		SilenceErrors: true, // Handled manually.
		Version:       version,
		PersistentPreRunE: func(cmd *Command, args []string) error {
			plugins := launchr.GetPluginByType[PersistentPreRunPlugin](app.pluginMngr)
			Log().Debug("hook PersistentPreRunPlugin", "plugins", plugins)
			for _, p := range plugins {
				if err := p.V.PersistentPreRun(cmd, args); err != nil {
					Log().Error("error on PersistentPreRunPlugin", "plugin", p.K.String(), "err", err)
					return err
				}
			}
			return nil
		},
		RunE: func(cmd *Command, _ []string) error {
			return cmd.Help()
		},
	}
	app.cmd.SetVersionTemplate(`{{ appVersionFull }}`)
	app.earlyCmd = launchr.EarlyPeekCommand()
	// Set io streams.
	app.SetStreams(MaskedStdStreams(app.SensitiveMask()))
	app.cmd.SetIn(app.streams.In().Reader())
	app.cmd.SetOut(app.streams.Out())
	app.cmd.SetErr(app.streams.Err())

	// Set working dir and config dir.
	app.cfgDir = "." + name
	app.workDir = launchr.MustAbs(".")
	actionsPath := launchr.MustAbs(EnvVarActionsPath.Get())
	// Initialize managed FS for action discovery.
	app.mFS = make([]ManagedFS, 0, 4)
	app.RegisterFS(action.NewDiscoveryFS(os.DirFS(actionsPath), app.GetWD()))

	// Prepare dependencies.
	app.services = make(map[ServiceInfo]Service)
	app.pluginMngr = launchr.NewPluginManagerWithRegistered()
	// @todo consider home dir for global config.
	config := launchr.ConfigFromFS(os.DirFS(app.cfgDir))
	actionMngr := action.NewManager(
		action.WithDefaultRuntime(config),
		action.WithContainerRuntimeConfig(config, name+"_"),
	)

	// Register services for other modules.
	app.AddService(actionMngr)
	app.AddService(app.pluginMngr)
	app.AddService(config)

	Log().Debug("initialising application")

	// Run OnAppInit hook.
	plugins := launchr.GetPluginByType[OnAppInitPlugin](app.pluginMngr)
	Log().Debug("hook OnAppInitPlugin", "plugins", plugins)
	for _, p := range plugins {
		if err = p.V.OnAppInit(app); err != nil {
			Log().Error("error on OnAppInit", "plugin", p.K.String(), "err", err)
			return err
		}
	}
	Log().Debug("init success", "wd", app.workDir, "actions_dir", actionsPath)

	return nil
}

func (app *appImpl) exec() error {
	Log().Debug("executing command")
	if app.earlyCmd.IsVersion {
		// Version is requested, no need to bootstrap further.
		return app.cmd.Execute()
	}

	// Add application commands from plugins.
	plugins := launchr.GetPluginByType[CobraPlugin](app.pluginMngr)
	Log().Debug("hook CobraPlugin", "plugins", plugins)
	for _, p := range plugins {
		if err := p.V.CobraAddCommands(app.cmd); err != nil {
			Log().Error("error on CobraAddCommands", "plugin", p.K.String(), "err", err)
			return err
		}
	}

	err := app.cmd.Execute()
	if err != nil {
		Log().Debug("execution error", "err", err)
	}

	return err
}

// Execute is an entrypoint to the launchr app.
func (app *appImpl) Execute() int {
	defer func() {
		Log().Debug("shutdown cleanup")
		if err := launchr.Cleanup(); err != nil {
			Term().Warning().Printfln("Error on application shutdown cleanup:\n %s", err)
		}
	}()
	var err error
	if err = app.init(); err != nil {
		Term().Error().Println(err)
		return 125
	}
	if err = app.exec(); err != nil {
		var status int
		var errExit ExitError

		switch {
		case errors.As(err, &errExit):
			status = errExit.ExitCode()
		default:
			status = 1
		}
		msg := err.Error()
		if msg != "" {
			Term().Error().Println(err)
		}

		return status
	}

	return 0
}

// Run executes the application.
func Run() int {
	return newApp().Execute()
}

// RunAndExit runs the application and exits with a result code.
func RunAndExit() {
	os.Exit(Run())
}
