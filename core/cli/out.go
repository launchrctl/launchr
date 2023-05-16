// Package cli implements printing functionality for CLI output.
package cli

import "fmt"

// Print formats a string.
func Print(format string, a ...any) {
	fmt.Printf(format, a...)
}

// Println formats a string and adds a new line.
func Println(format string, a ...any) {
	fmt.Println(fmt.Sprintf(format, a...))
}
