// Package executes Launchr application.
//
//go:generate go run ./gen.go ../../
package main

import (
	"os"

	"github.com/launchrctl/launchr"
)

func main() {
	os.Exit(launchr.Run())
}
