// Package launchr has application implementation.
package launchr

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"text/template"

	"github.com/spf13/cobra"

	"github.com/launchrctl/launchr/core/cli"
	"github.com/launchrctl/launchr/core/log"
)

// App holds app related global variables.
type App struct {
	cmd     *cobra.Command
	cli     cli.Cli
	wdFS    fs.FS
	version fmt.Stringer
	plugins map[PluginInfo]Plugin
}

// NewApp constructs app implementation.
func NewApp() *App {
	return &App{}
}

// GetFS returns application wdFS.
func (app *App) GetFS() fs.FS {
	return app.wdFS
}

// SetFS sets application wdFS.
func (app *App) SetFS(f fs.FS) {
	app.wdFS = f
}

// GetCli returns application cli.
func (app *App) GetCli() cli.Cli {
	return app.cli
}

// SetCli sets application cli.
func (app *App) SetCli(c cli.Cli) {
	app.cli = c
}

// SetVersion sets application cli.
func (app *App) SetVersion(v fmt.Stringer) {
	app.version = v
}

// Version returns application version string.
func (app *App) Version() string {
	return app.version.String()
}

// Plugins returns installed app plugins.
func (app *App) Plugins() map[PluginInfo]Plugin {
	return app.plugins
}

// Init initializes application and plugins.
func (app *App) Init() error {
	// Global configuration.
	appCli, err := cli.NewAppCli(
		cli.WithStandardStreams(),
		cli.WithGlobalConfigFromDir(os.DirFS(".launchr")), // @todo how should it work with embed?
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

func (app *App) gen(buildPath string, wordDir string) error {
	var err error
	buildPath, err = filepath.Abs(buildPath)
	if err != nil {
		return err
	}
	wordDir, err = filepath.Abs(wordDir)
	if err != nil {
		return err
	}
	// Clean build path before generating.
	err = filepath.WalkDir(buildPath, func(path string, dir fs.DirEntry, err error) error {
		if path == buildPath {
			return nil
		}
		if dir.IsDir() {
			errRem := os.RemoveAll(path)
			if errRem != nil {
				return errRem
			}
			return filepath.SkipDir
		}
		if dir.Name() != "pkg.go" {
			errRem := os.Remove(path)
			if errRem != nil {
				return errRem
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	// Call generate functions on plugins.
	plugins := app.Plugins()
	initSet := make(map[string]struct{}, len(plugins))
	for _, p := range app.Plugins() {
		p, ok := p.(GeneratePlugin)
		if ok {
			genData, err := p.Generate(buildPath, wordDir)
			if err != nil {
				return err
			}
			for _, class := range genData.Plugins {
				initSet[class] = struct{}{}
			}
		}
	}
	if len(initSet) > 0 {
		var tplName = "init.gen"
		tpl, err := template.New(tplName).Parse(initGenTemplate)
		if err != nil {
			return err
		}
		// Generate main.go.
		var buf bytes.Buffer
		err = tpl.Execute(&buf, &initTplVars{
			Plugins: initSet,
		})
		if err != nil {
			return err
		}
		target := filepath.Join(buildPath, tplName+".go")
		err = os.WriteFile(target, buf.Bytes(), 0600)
		if err != nil {
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
	if app.version != nil {
		rootCmd.SetVersionTemplate(app.Version())
	}
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

// Generate runs generation of included plugins.
func (app *App) Generate() int {
	buildPath := "./gen"
	wd := "./"
	if len(os.Args) > 1 {
		wd = os.Args[1]
	}
	if err := app.gen(buildPath, wd); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 125
	}
	return 0
}

// Execute is a cobra entrypoint to the launchr app.
func (app *App) Execute() int {
	app.version = GetVersion()
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

// Gen generates application specific build files and returns os exit code.
func Gen() int {
	app := NewApp()
	if err := app.Init(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 125
	}
	return app.Generate()
}

type initTplVars struct {
	Plugins map[string]struct{}
}

const initGenTemplate = `
{{- print "// GENERATED. DO NOT EDIT." }}
package gen

import (
	"github.com/launchrctl/launchr"
)

func init() {
	{{- range $class, $ := .Plugins }}
	launchr.RegisterPlugin(&{{$class}}{})
	{{- end }}
}
`
