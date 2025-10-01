package testactions

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/launchrctl/launchr/internal/launchr"
	"github.com/launchrctl/launchr/pkg/action"
)

const sensitiveYaml = `
runtime: plugin
action:
  title: Test Plugin - Sensitive mask
  arguments:
    - name: arg
      required: true
`

func actionSensitive(app launchr.App, mask *launchr.SensitiveMask) *action.Action {
	// Create an action that outputs a secret in a terminal.
	secret := os.Getenv("TEST_SECRET")
	if secret != "" {
		mask.AddString(secret)
	}

	a := action.NewFromYAML("testplugin:sensitive", []byte(sensitiveYaml))
	a.SetRuntime(action.NewFnRuntime(func(_ context.Context, a *action.Action) error {
		arg := a.Input().Arg("arg").(string)
		streams := app.Streams()
		// Check terminal.
		launchr.Term().Printfln("terminal output: %s", arg)
		// Check log.
		launchr.Log().Error("log output: " + arg)
		// Check raw.
		fmt.Printf("fmt print: %s\n", arg)
		// Check using streams.
		_, _ = fmt.Fprintf(streams.Out(), "fmt stdout streams print: %s\n", arg)
		_, _ = fmt.Fprintf(streams.Err(), "fmt stderr streams print: %s\n", arg)
		// Check if we output by parts.
		parts := strings.Split(arg, " ")
		if len(parts) == 2 {
			launchr.Term().Print("split output: ")
			launchr.Term().Print(parts[0])
			launchr.Term().Print(" ")
			launchr.Term().Println(parts[1])
		}
		return nil
	}))
	return a
}
