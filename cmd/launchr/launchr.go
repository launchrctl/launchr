// Package executes Launchr application.
package main

import (
	"os"

	"github.com/launchrctl/launchr"
	_ "github.com/launchrctl/launchr/cmd/launchr/gen"
	_ "github.com/launchrctl/launchr/core/plugins"
)

func main() {
	os.Exit(launchr.Run())
}
