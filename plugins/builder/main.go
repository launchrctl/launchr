// Package main implements entrypoint to launchr.
package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/launchrctl/launchr/core"
	"github.com/launchrctl/launchr/core/builder"
)

var (
	Name      = "launchr" // Name - version info
	Version   = "dev"     // Version - version info
	GoVersion string      // GoVersion - version info
	BuildDate string      // BuildDate - version info
	GitHash   string      // GitHash - version info
	GitBranch string      // GitBranch - version info
)

func execute() int {
	v := &core.AppVersion{
		Name:      Name,
		Version:   Version,
		GoVersion: GoVersion,
		BuildDate: BuildDate,
		GitHash:   GitHash,
		GitBranch: GitBranch,
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
	}

	if err := builder.Execute(v); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	return 0
}

func main() {
	os.Exit(execute())
}
