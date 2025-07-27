package testactions

import (
	"context"

	"github.com/launchrctl/launchr/internal/launchr"
	"github.com/launchrctl/launchr/pkg/action"
)

const logLevelsYaml = `
runtime: plugin
action:
  title: Test Plugin - Log levels
`

func actionLogLevels() *action.Action {
	// Create an action that outputs to log all message types.
	a := action.NewFromYAML("testplugin:log-levels", []byte(logLevelsYaml))
	a.SetRuntime(action.NewFnRuntime(func(_ context.Context, _ *action.Action) error {
		launchr.Log().Debug("this is DEBUG log")
		launchr.Log().Info("this is INFO log")
		launchr.Log().Warn("this is WARN log")
		launchr.Log().Error("this is ERROR log")
		return nil
	}))
	return a
}
