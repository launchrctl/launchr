package testactions

import (
	"context"
	"fmt"
	"strings"

	"github.com/launchrctl/launchr/pkg/action"
)

// pluginPrintInput adds to all actions prefixed with "test-print-input:"
// a special runtime that outputs the given input.
func pluginPrintInput(_ action.Manager, a *action.Action) {
	if !strings.HasPrefix(a.ID, "test-print-input:") {
		return
	}
	a.SetRuntime(action.NewFnRuntime(func(_ context.Context, a *action.Action) error {
		def := a.ActionDef()
		for _, p := range def.Arguments {
			printParam(p.Name, a.Input().Arg(p.Name), a.Input().IsArgChanged(p.Name))
		}
		for _, p := range def.Options {
			printParam(p.Name, a.Input().Opt(p.Name), a.Input().IsOptChanged(p.Name))
		}
		return nil
	}))
}

func printParam(name string, val any, isChanged bool) {
	fmt.Printf("%s: %v %T %t\n", name, val, val, isChanged)
}
