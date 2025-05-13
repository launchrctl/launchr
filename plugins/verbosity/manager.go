package verbosity

import (
	"github.com/launchrctl/launchr/internal/launchr"
	"github.com/launchrctl/launchr/pkg/action"
	"github.com/launchrctl/launchr/pkg/jsonschema"
)

// withCustomLogger decorator adds a default [Runtime] for an action.
func withCustomLogger(_ action.Manager, a *action.Action) {
	if a.Runtime() == nil {
		return
	}

	if !a.Input().IsValidated() {
		return
	}

	if rt, ok := a.Runtime().(action.RuntimeLoggerAware); ok {
		var logFormat LogFormat
		if lfStr, ok := a.Input().PersistentFlag("log-format").(string); ok {
			logFormat = LogFormat(lfStr)
		}

		var logLevel launchr.LogLevel
		if llStr, ok := a.Input().PersistentFlag("log-level").(string); ok {
			logLevel = launchr.LogLevelFromString(llStr)
		}

		logger := NewLogger(logFormat, logLevel, a.Input().Streams().Out())
		rt.SetLogger(logger)
	}
}

// withCustomTerm decorator adds a default [Runtime] for an action.
func withCustomTerm(_ action.Manager, a *action.Action) {
	if a.Runtime() == nil {
		return
	}

	if !a.Input().IsValidated() {
		return
	}

	if rt, ok := a.Runtime().(action.RuntimeTermAware); ok {
		term := launchr.NewTerminal()
		term.SetOutput(a.Input().Streams().Out())
		if quiet, ok := a.Input().PersistentFlag("log-level").(bool); ok && quiet {
			term.DisableOutput()
		}

		rt.SetTerm(term)
	}
}

func (p Plugin) getPluginPersistentFlags() action.ParametersList {
	return action.ParametersList{
		&action.DefParameter{
			Name:    "log-level",
			Title:   "log-level",
			Type:    jsonschema.String,
			Default: "NONE",
			Enum:    []any{"DEBUG", "INFO", "WARN", "ERROR", "NONE"},
		},
		&action.DefParameter{
			Name:    "log-format",
			Title:   "log-format",
			Type:    jsonschema.String,
			Default: "pretty",
			Enum:    []any{"pretty", "plain", "json"},
		},
		&action.DefParameter{
			Name:        "quiet",
			Title:       "quiet",
			Description: "disable output to the console",
			Type:        jsonschema.Boolean,
			Default:     false,
		},
	}
}
