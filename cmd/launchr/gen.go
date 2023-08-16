//go:build ignore

package main

import (
	"os"

	"github.com/launchrctl/launchr"
)

func main() {
	os.Exit(launchr.Gen())
}
