package launchr

import (
	"os"
	"path/filepath"
)

func (app *appImpl) gen(buildPath string, workDir string) error {
	var err error
	buildPath, err = filepath.Abs(buildPath)
	if err != nil {
		return err
	}
	workDir, err = filepath.Abs(workDir)
	if err != nil {
		return err
	}

	// Call generate functions on plugins.
	for _, p := range getPluginByType[GeneratePlugin](app) {
		err = p.Generate(buildPath, workDir)
		if err != nil {
			return err
		}
	}

	return nil
}

// Generate runs generation of included plugins.
func (app *appImpl) Generate() int {
	buildPath := "./"
	wd := buildPath
	if len(os.Args) > 1 {
		wd = os.Args[1]
	}
	var err error
	if err = app.init(); err != nil {
		Term().Error().Println(err)
		return 125
	}
	if err = app.gen(buildPath, wd); err != nil {
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
