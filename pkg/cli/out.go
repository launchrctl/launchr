// Package cli implements printing functionality for CLI output.
package cli

import (
	"github.com/launchrctl/launchr/internal/launchr"
)

// Print formats a string.
// Deprecated: use `launchr.Term().Printf()` or `launchr.Term().Info().Printf()`
func Print(format string, a ...any) {
	launchr.Term().Printf(format, a...)
}

// Println formats a string and adds a new line.
// Deprecated: use `launchr.Term().Printfln()` or `launchr.Term().Info().Printfln()`
func Println(format string, a ...any) {
	launchr.Term().Printfln(format, a...)
}
