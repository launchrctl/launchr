//go:build ignore

package main

import (
	"os"

	"github.com/launchrctl/launchr"
	_ "github.com/launchrctl/launchr/pkg/plugins"
)

func main() {
	os.Exit(launchr.Gen())
}
