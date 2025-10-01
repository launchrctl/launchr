package launchr

import (
	"os"
	"strings"

	"github.com/launchrctl/launchr/internal/launchr"
)

func (app *appImpl) gen() error {
	var err error
	config := GenerateConfig{
		WorkDir:  ".",
		BuildDir: ".",
	}
	isRelease := false
	app.cmd.RunE = func(cmd *Command, _ []string) error {
		// Don't show usage help on a runtime error.
		cmd.SilenceUsage = true
		// Set absolute paths.
		config.WorkDir = launchr.MustAbs(config.WorkDir)
		config.BuildDir = launchr.MustAbs(config.BuildDir)
		// Change the working directory to the selected.
		err = os.Chdir(config.WorkDir)
		if err != nil {
			return err
		}

		// Call generate functions on plugins.
		plugins := launchr.GetPluginByType[GeneratePlugin](app.pluginMngr)
		Log().Debug("hook GeneratePlugin", "plugins", plugins)
		for _, p := range plugins {
			if !isRelease && strings.HasPrefix(p.K.GetPackagePath(), PkgPath) {
				// Skip core packages if not requested.
				// Implemented for development of plugins to prevent generating of main.go.
				continue
			}
			err = p.V.Generate(config)
			if err != nil {
				Log().Error("error on Generate", "plugin", p.K.String(), "err", err)
				return err
			}
		}
		return nil
	}

	// Do not fail if some flags change in future builds.
	flags := app.cmd.Flags()
	flags.ParseErrorsAllowlist.UnknownFlags = true
	// Working directory flag is helpful because "go run" can't be run outside
	// a project directory and use all its go.mod dependencies.
	flags.StringVar(&config.WorkDir, "work-dir", config.WorkDir, "Working directory")
	flags.StringVar(&config.BuildDir, "build-dir", config.BuildDir, "Build directory where the files will be generated")
	flags.BoolVar(&isRelease, "release", isRelease, "Generate core plugins and main.go")

	return app.cmd.Execute()
}

// Generate runs generation of included plugins.
func (app *appImpl) Generate() int {
	defer func() {
		if err := launchr.Cleanup(); err != nil {
			Term().Warning().Printfln("Error on application shutdown cleanup:\n %s", err)
		}
	}()
	var err error
	if err = app.init(); err != nil {
		Term().Error().Println(err)
		return 125
	}
	if err = app.gen(); err != nil {
		Term().Error().Println(err)
		return 125
	}
	return 0
}

// Gen generates application specific build files and returns os exit code.
func Gen() int {
	// Do not discover actions on generate.
	launchr.IsGen = true
	return newApp().Generate()
}

// GenAndExit runs the generation and exits with a result code.
func GenAndExit() {
	os.Exit(Gen())
}
