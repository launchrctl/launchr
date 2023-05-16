//go:build ignore

package main

// @todo review
import (
	"fmt"
	"os"

	_ "github.com/launchrctl/launchr/plugins/yamldiscovery"

	launchr "github.com/launchrctl/launchr/core"
)

func main() {
	app := launchr.NewApp()
	if err := app.Init(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(125)
	}
	os.Exit(app.Generate())
}
