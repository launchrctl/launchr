package launchr

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/launchrctl/launchr/internal/launchr"
	"github.com/launchrctl/launchr/pkg/action"
	"github.com/launchrctl/launchr/pkg/cli"
	_ "github.com/launchrctl/launchr/plugins" // include default plugins
)

var (
	errTplAssetsNotFound = "assets not found for requested plugin %s"
)

// ActionsGroup is a cobra command group definition
var ActionsGroup = &cobra.Group{
	ID:    "actions",
	Title: "Actions:",
}

type launchrCfg struct {
	ActionsNaming []struct {
		Search  string `yaml:"search"`
		Replace string `yaml:"replace"`
	} `yaml:"actions_naming"`
}

type appImpl struct {
	rootCmd       *cobra.Command
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

	// Quick parse arguments to see if a version or help was requested.
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		// Skip discover actions if we check version.
		if args[i] == "--version" {
			app.skipActions = true
		}

		if app.reqCmd == "" && !strings.HasPrefix(args[i], "-") {
			app.reqCmd = args[i]
		}
	}

	// Discover actions.
	if !app.skipActions {
		var launchrConfig *launchrCfg
		err = app.config.Get("launchrctl", &launchrConfig)
		if err != nil {
			return err
		}

		for _, p := range getPluginByType[ActionDiscoveryPlugin](app) {
			for _, fs := range app.GetRegisteredFS() {
				actions, err := p.DiscoverActions(fs)
				if err != nil {
					return err
				}
				for _, actConf := range actions {
					if err = actConf.EnsureLoaded(); err != nil {
						return err
					}

					if launchrConfig != nil && len(launchrConfig.ActionsNaming) > 0 {
						actID := actConf.ID
						for _, an := range launchrConfig.ActionsNaming {
							actID = strings.ReplaceAll(actID, an.Search, an.Replace)
						}
						actConf.ID = actID
					}

					app.actionMngr.Add(actConf)
				}
			}
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

	if app.skipActions {
		rootCmd.SetVersionTemplate(Version().Full())
	}
	// Convert actions to cobra commands.
	actions := app.actionMngr.AllRef()
	// Check the requested command to see what actions we must actually load.
	if app.reqCmd != "" {
		aliases := app.actionMngr.AllAliasRef()
		if alias, ok := aliases[app.reqCmd]; ok {
			app.reqCmd = alias
		}
		a, ok := actions[app.reqCmd]
		if ok {
			// Use only the requested action.
			actions = map[string]*action.Action{a.ID: a}
		} else {
			// Action was not requested, no need to load them.
			app.skipActions = true
		}
	}
	// @todo consider cobra completion and caching between runs.
	if !app.skipActions {
		if len(actions) > 0 {
			rootCmd.AddGroup(ActionsGroup)
		}
		for _, a := range actions {
			a = app.actionMngr.Decorate(a)
			if err := a.EnsureLoaded(); err != nil {
				fmt.Fprintf(os.Stdout, "[WARNING] Action %q was skipped because it has an incorrect definition:\n%v\n", a.ID, err)
				continue
			}
			cmd, err := action.CobraImpl(a, app.Streams())
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
func Run(options *AppOptions) int {
	return newApp(options).Execute()
}
