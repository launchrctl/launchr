package launchr

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/launchrctl/launchr/internal/launchr"
	"github.com/launchrctl/launchr/pkg/action"
	"github.com/launchrctl/launchr/pkg/cli"
	_ "github.com/launchrctl/launchr/plugins" // include default plugins
)

var (
	errTplAssetsNotFound = "assets not found for requested plugin %s"
	errDiscoveryTimeout  = "action discovery timeout exceeded"
)

// ActionsGroup is a cobra command group definition.
var ActionsGroup = &cobra.Group{
	ID:    "actions",
	Title: "Actions:",
}

type appImpl struct {
	cmd           *cobra.Command
	streams       cli.Streams
	workDir       string
	cfgDir        string
	services      map[ServiceInfo]Service
	actionMngr    action.Manager
	pluginMngr    PluginManager
	config        Config
	mFS           []ManagedFS
	skipActions   bool   // skipActions to skip loading if not requested.
	reqCmd        string // reqCmd to search for the requested cobra command.
	assetsStorage embed.FS
}

// AppOptions represents the launchr application options.
type AppOptions struct {
	AssetsFs embed.FS
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

func newApp(options *AppOptions) *appImpl {
	return &appImpl{assetsStorage: options.AssetsFs}
}

func (app *appImpl) Name() string         { return name }
func (app *appImpl) GetWD() string        { return app.workDir }
func (app *appImpl) Streams() cli.Streams { return app.streams }

func (app *appImpl) RegisterFS(fs ManagedFS)      { app.mFS = append(app.mFS, fs) }
func (app *appImpl) GetRegisteredFS() []ManagedFS { return app.mFS }

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

func (app *appImpl) GetPluginAssets(p Plugin) fs.FS {
	pluginsMap := app.pluginMngr.All()
	var packagePath string
	for pi, plg := range pluginsMap {
		if plg == p {
			packagePath = pi.GetPackagePath()
		}
	}

	if packagePath == "" {
		panic(errors.New("trying to get assets for unknown plugin"))
	}

	subFS, err := fs.Sub(app.assetsStorage, filepath.Join("assets", packagePath))
	if err != nil {
		panic(fmt.Errorf(errTplAssetsNotFound, packagePath))
	}

	return subFS
}

// earlyPeekFlags tries to parse flags early to allow change behavior before cobra has booted.
func (app *appImpl) earlyPeekFlags(c *cobra.Command) {
	args := os.Args[1:]
	// Parse args with cobra.
	// We can't guess cmd because nothing has been defined yet.
	// We don't care about error because there won't be any on clean cmd.
	_, flags, _ := c.Find(args)
	// Quick parse arguments to see if a version or help was requested.
	for i := 0; i < len(flags); i++ {
		// Skip discover actions if we check version.
		if flags[i] == "--version" {
			app.skipActions = true
		}

		if app.reqCmd == "" && !strings.HasPrefix(flags[i], "-") {
			app.reqCmd = args[i]
		}
	}
}

// init initializes application and plugins.
func (app *appImpl) init() error {
	var err error
	// Set root cobra command.
	app.cmd = &cobra.Command{
		Use: name,
		//Short: "", // @todo
		//Long:  ``, // @todo
		SilenceErrors: true, // Handled manually.
		Version:       version,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	app.earlyPeekFlags(app.cmd)

	// Set io streams.
	app.streams = cli.StandardStreams()
	app.cmd.SetIn(app.streams.In())
	app.cmd.SetOut(app.streams.Out())
	app.cmd.SetErr(app.streams.Err())

	// Set working dir and config dir.
	app.cfgDir = "." + name
	app.workDir, err = filepath.Abs(".")
	if err != nil {
		return err
	}
	// Initialize managed FS for action discovery.
	app.mFS = make([]ManagedFS, 0, 4)
	app.RegisterFS(action.NewDiscoveryFS(os.DirFS(app.workDir), app.GetWD()))

	// Prepare dependencies.
	app.services = make(map[ServiceInfo]Service)
	app.pluginMngr = launchr.NewPluginManagerWithRegistered()
	// @todo consider home dir for global config.
	app.config = launchr.ConfigFromFS(os.DirFS(app.cfgDir))
	app.actionMngr = action.NewManager(
		action.WithDefaultRunEnvironment,
		action.WithContainerRunEnvironmentConfig(app.config, name+"_"),
		action.WithValueProcessors(),
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
	if !app.skipActions {
		if err = app.discoverActions(); err != nil {
			return err
		}
	}

	return nil
}

func (app *appImpl) discoverActions() (err error) {
	var discovered []*action.Action
	idp := app.actionMngr.GetActionIDProvider()
	// @todo configure from flags
	// Define timeout for cases when we may traverse the whole FS, e.g. in / or home.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for _, p := range getPluginByType[action.DiscoveryPlugin](app) {
		for _, fs := range app.GetRegisteredFS() {
			actions, errDis := p.DiscoverActions(ctx, fs, idp)
			if errDis != nil {
				return errDis
			}
			discovered = append(discovered, actions...)
		}
	}
	// Failed to discover actions in reasonable time.
	if errCtx := ctx.Err(); errCtx != nil {
		return errors.New(errDiscoveryTimeout)
	}

	// Add discovered actions.
	for _, a := range discovered {
		app.actionMngr.Add(a)
	}

	// Alter all registered actions.
	for _, p := range getPluginByType[action.AlterActionsPlugin](app) {
		err = p.AlterActions()
		if err != nil {
			return err
		}
	}
	// @todo maybe cache discovery result for performance.
	return err
}

func (app *appImpl) exec() error {
	if app.skipActions {
		app.cmd.SetVersionTemplate(Version().Full())
	}
	// Check the requested command to see what actions we must actually load.
	var actions map[string]*action.Action
	if app.reqCmd != "" {
		// Check if an alias was provided to find the real action.
		app.reqCmd = app.actionMngr.GetIDFromAlias(app.reqCmd)
		a, ok := app.actionMngr.Get(app.reqCmd)
		if ok {
			// Use only the requested action.
			actions = map[string]*action.Action{a.ID: a}
		} else {
			// Action was not requested, no need to load them.
			app.skipActions = true
		}
	} else {
		// Load all.
		actions = app.actionMngr.All()
	}
	// Convert actions to cobra commands.
	// @todo consider cobra completion and caching between runs.
	if !app.skipActions {
		if len(actions) > 0 {
			app.cmd.AddGroup(ActionsGroup)
		}
		for _, a := range actions {
			cmd, err := action.CobraImpl(a, app.Streams())
			if err != nil {
				fmt.Fprintf(os.Stdout, "[WARNING] Action %q was skipped:\n%v\n", a.ID, err)
				continue
			}
			cmd.GroupID = ActionsGroup.ID
			app.cmd.AddCommand(cmd)
		}
	}

	// Add cobra commands from plugins.
	for _, p := range getPluginByType[CobraPlugin](app) {
		if err := p.CobraAddCommands(app.cmd); err != nil {
			return err
		}
	}

	return app.cmd.Execute()
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
func Run(options *AppOptions) int {
	return newApp(options).Execute()
}
