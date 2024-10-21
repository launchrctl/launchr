package launchr

import (
	"os"
	"path/filepath"
	"strings"
)

func (app *appImpl) gen() error {
	var err error
	config := GenerateConfig{
		WorkDir:  ".",
		BuildDir: ".",
	}
	isRelease := false
	app.cmd.RunE = func(_ *Command, args []string) error {
		if len(args) > 0 {
			// Save backward compatibility with the previous build implementation.
			// @todo delete after release.
			config.WorkDir = args[0]
		}
		// Set absolute paths.
		config.WorkDir, err = filepath.Abs(config.WorkDir)
		if err != nil {
			return err
		}
		config.BuildDir, err = filepath.Abs(config.BuildDir)
		if err != nil {
			return err
		}
		// Change working directory to the selected.
		err = os.Chdir(config.WorkDir)
		if err != nil {
			return err
		}

		// Call generate functions on plugins.
		for _, p := range getPluginByType[GeneratePlugin](app) {
			if !isRelease && strings.HasPrefix(p.K.GetPackagePath(), PkgPath) {
				// Skip core packages if not requested.
				// Implemented for development of plugins to prevent generating of main.go.
				continue
			}
			err = p.V.Generate(config)
			if err != nil {
				return err
			}
		}
		return nil
	}

	// Do not fail if some flags change in future builds.
	flags := app.cmd.Flags()
	flags.ParseErrorsWhitelist.UnknownFlags = true
	// Working directory flag is helpful because "go run" can't be run outside
	// a project directory and use all its go.mod dependencies.
	flags.StringVar(&config.WorkDir, "work-dir", config.WorkDir, "Working directory")
	flags.StringVar(&config.BuildDir, "build-dir", config.BuildDir, "Build directory where the files will be generated")
	flags.BoolVar(&isRelease, "release", isRelease, "Generate core plugins and main.go")

	return app.cmd.Execute()
}

// Generate runs generation of included plugins.
func (app *appImpl) Generate() int {
	// Do not discover actions on generate.
	app.skipActions = true
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
	return newApp().Generate()
}

// GenAndExit runs the generation and exits with a result code.
func GenAndExit() {
	os.Exit(Gen())
}
