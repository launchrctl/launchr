// Package executes Launchr application.
//
//go:generate go run ./gen.go ../../
package main

import (
	"os"

	"github.com/launchrctl/launchr"
	_ "github.com/launchrctl/launchr/cmd/launchr/gen"
)

func main() {
	os.Exit(launchr.Run(&launchr.AppOptions{}))
}
