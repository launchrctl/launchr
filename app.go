package launchr

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/launchrctl/launchr/pkg/jsonschema"

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
	mFS        []ManagedFS
	processors map[string]ValueProcessor
}

// ProcessorCallback is a function signature used as a callback in processors.
type ProcessorCallback func(value interface{}, options map[string]interface{}) (interface{}, error)

// FuncProcessor represents a processor that applies a callback function to values based on certain applicable formats.
// It has two fields: applicableFormats and callback.
//
// applicableFormats is a slice of jsonschema.Type, which defines the formats that this processor is applicable to.
//
// callback is a ProcessorCallback function that takes a value of any type and options as a map[string]interface{},
// and returns a processed value and an error.
// If the callback is successfully executed, the processed value and nil error are returned.
// Otherwise, an error is returned.
type FuncProcessor struct {
	applicableFormats []jsonschema.Type
	callback          ProcessorCallback
}

// NewFuncProcessor creates a new instance of FuncProcessor with the specified formats and callback.
//
// Parameter formats is a slice of jsonschema.Type representing the applicable formats.
//
// Parameter callback is a ProcessorCallback function that takes a value and options map as input,
// and returns a processed value and an error.
//
// Returns a FuncProcessor instance initialized with the given formats and callback.
func NewFuncProcessor(formats []jsonschema.Type, callback ProcessorCallback) FuncProcessor {
	return FuncProcessor{
		applicableFormats: formats,
		callback:          callback,
	}
}

// IsApplicable checks if the given valueType is present in the applicableFormats slice of the FuncProcessor.
// It returns true if the valueType is found, otherwise it returns false.
func (p FuncProcessor) IsApplicable(valueType jsonschema.Type) bool {
	for _, item := range p.applicableFormats {
		if valueType == item {
			return true
		}
	}

	return false
}

// Execute applies the callback function of the FuncProcessor to the given value and options.
// It returns the result of the callback function and any error that occurred during execution.
func (p FuncProcessor) Execute(value interface{}, options map[string]interface{}) (interface{}, error) {
	return p.callback(value, options)
}

// getPluginByType returns specific plugins from the app.
func getPluginByType[T Plugin](app *appImpl) []T {
	plugins := app.pluginMngr.All()
	res := make([]T, 0, len(plugins))
	// Collect plugins according to their weights.
	m := make(map[int][]T)
	for pi, p := range plugins {
		p, ok := p.(T)
		if ok {
			m[pi.Weight] = append(m[pi.Weight], p)
		}
	}
	// Sort weight keys.
	weights := make([]int, 0, len(m))
	for w := range m {
		weights = append(weights, w)
	}
	sort.Ints(weights)
	// Merge all to a sorted list of plugins.
	// @todo maybe sort everything on init to optimize.
	for _, w := range weights {
		res = append(res, m[w]...)
	}
	return res
}

func newApp() *appImpl {
	return &appImpl{}
}

func (app *appImpl) Name() string         { return name }
func (app *appImpl) GetWD() string        { return app.workDir }
func (app *appImpl) Streams() cli.Streams { return app.streams }

func (app *appImpl) RegisterFS(fs ManagedFS)      { app.mFS = append(app.mFS, fs) }
func (app *appImpl) GetRegisteredFS() []ManagedFS { return app.mFS }

func (app *appImpl) AddProcessor(name string, vp ValueProcessor) error {
	if app.processors == nil {
		app.processors = make(map[string]ValueProcessor)
	}

	if _, ok := app.processors[name]; ok {
		return errors.New("processor with the same name already exists")
	}

	app.processors[name] = vp

	return nil
}

func (app *appImpl) GetRegisteredProcessors() map[string]ValueProcessor {
	return app.processors
}

func (app *appImpl) AddService(s Service) {
	info := s.ServiceInfo()
	launchr.InitServiceInfo(&info, s)
	if _, ok := app.services[info]; ok {
		panic(fmt.Errorf("service %s already exists, review your code", info))
	}
	app.services[info] = s
}

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
	app.workDir, err = filepath.Abs(".")
	if err != nil {
		return err
	}
	app.mFS = make([]ManagedFS, 0, 4)
	app.RegisterFS(action.NewDiscoveryFS(os.DirFS(app.workDir), app.GetWD()))
	// Prepare dependencies.
	app.streams = cli.StandardStreams()
	app.services = make(map[ServiceInfo]Service)
	app.pluginMngr = launchr.NewPluginManagerWithRegistered()
	app.config = launchr.ConfigFromFS(os.DirFS(app.cfgDir))
	app.actionMngr = action.NewManager(
		action.WithDefaultRunEnvironment,
		action.WithContainerRunEnvironmentConfig(app.config, name+"_"),
	)

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

	// Discover actions.
	for _, p := range getPluginByType[ActionDiscoveryPlugin](app) {
		for _, fs := range app.GetRegisteredFS() {
			actions, errDiscover := p.DiscoverActions(fs)
			if errDiscover != nil {
				return errDiscover
			}
			for _, actConf := range actions {
				app.actionMngr.Add(actConf)
			}
		}
	}

	for _, p := range getPluginByType[ProcessorDiscoveryPlugin](app) {
		if err = p.DiscoverProcessors(app); err != nil {
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
	// Quick parse arguments to see if a version or help was requested.
	args := os.Args[1:]
	var skipActions bool // skipActions to skip loading if not requested.
	var reqCmd string    // reqCmd to search for the requested cobra command.
	for i := 0; i < len(args); i++ {
		if args[i] == "--version" {
			rootCmd.SetVersionTemplate(Version().Full())
			skipActions = true
		}
		if reqCmd == "" && !strings.HasPrefix(args[i], "-") {
			reqCmd = args[i]
		}
	}

	// Convert actions to cobra commands.
	actions := app.actionMngr.AllRef()
	// Check the requested command to see what actions we must actually load.
	if reqCmd != "" {
		a, ok := actions[reqCmd]
		if ok {
			// Use only the requested action.
			actions = map[string]*action.Action{a.ID: a}
		} else {
			// Action was not requested, no need to load them.
			skipActions = true
		}
	}
	// @todo consider cobra completion and caching between runs.
	if !skipActions {
		if len(actions) > 0 {
			rootCmd.AddGroup(ActionsGroup)
		}
		for _, a := range actions {
			a = app.actionMngr.Decorate(a)
			if err := a.EnsureLoaded(); err != nil {
				fmt.Fprintf(os.Stdout, "[WARNING] Action %q was skipped because it has an incorrect definition:\n%v\n", a.ID, err)
				continue
			}
			cmd, err := action.CobraImpl(a, app.Streams(), app.GetRegisteredProcessors())
			if err != nil {
				fmt.Fprintf(os.Stdout, "[WARNING] Action %q was skipped:\n%v\n", a.ID, err)
				continue
			}
			cmd.GroupID = ActionsGroup.ID
			rootCmd.AddCommand(cmd)
		}
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
		var status int
		var stErr action.RunStatusError

		switch {
		case errors.As(err, &stErr):
			status = stErr.GetCode()
		default:
			status = 1
			fmt.Fprintln(os.Stderr, "Error:", err)
		}

		return status
	}

	return 0
}

// Run executes launchr application.
func Run() int {
	return newApp().Execute()
}
