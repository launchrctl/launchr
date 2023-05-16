// Package executes Launchr application.
package main

import (
	"fmt"
	"os"
	"runtime"

	_ "github.com/launchrctl/launchr/plugins/yamldiscovery"

	launchr "github.com/launchrctl/launchr/core"
)

var (
	Name      = "launchr" // Name - version info
	Version   = "dev"     // Version - version info
	GoVersion string      // GoVersion - version info
	BuildDate string      // BuildDate - version info
	GitHash   string      // GitHash - version info
	GitBranch string      // GitBranch - version info
)

func main() {
	v := &launchr.AppVersion{
		Name:      Name,
		Version:   Version,
		GoVersion: GoVersion,
		BuildDate: BuildDate,
		GitHash:   GitHash,
		GitBranch: GitBranch,
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
	}

	app := launchr.NewApp()
	app.SetVersion(v)
	if err := app.Init(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(125)
	}
	os.Exit(app.Execute())
}
