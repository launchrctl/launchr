// Package verbosity is a plugin of launchr to configure log level of the app.
package verbosity

import (
	"fmt"
	"io"
	"math"

	"github.com/launchrctl/launchr/internal/launchr"
	"github.com/launchrctl/launchr/pkg/action"
	"github.com/launchrctl/launchr/pkg/jsonschema"
)

func init() {
	launchr.RegisterPlugin(&Plugin{})
}

// Plugin is [launchr.Plugin] to set verbosity of the application.
type Plugin struct{}

// PluginInfo implements [launchr.Plugin] interface.
func (p Plugin) PluginInfo() launchr.PluginInfo {
	return launchr.PluginInfo{
		Weight: math.MinInt, // Ensure to be run first.
	}
}

// LogFormat is a enum type for log output format.
type LogFormat string

// Log formats.
const (
	LogFormatPretty LogFormat = "pretty" // LogFormatPretty is a default logger output format.
	LogFormatPlain  LogFormat = "plain"  // LogFormatPlain is a plain logger output format.
	LogFormatJSON   LogFormat = "json"   // LogFormatJSON is a json logger output format.
)

// Set implements [fmt.Stringer] interface.
func (e *LogFormat) String() string {
	return string(*e)
}

// Set implements [github.com/spf13/pflag.Value] interface.
func (e *LogFormat) Set(v string) error {
	lf := LogFormat(v)
	switch lf {
	case LogFormatPlain, LogFormatJSON, LogFormatPretty:
		*e = lf
		return nil
	default:
		return fmt.Errorf(
			`must be one of %s, %s, %s`,
			LogFormatPlain, LogFormatJSON, LogFormatPretty,
		)
	}
}

// Type implements [github.com/spf13/pflag.Value] interface.
func (e *LogFormat) Type() string {
	return "LogFormat"
}

type logLevelStr string

// Set implements [fmt.Stringer] interface.
func (e *logLevelStr) String() string {
	return string(*e)
}

// Set implements [github.com/spf13/pflag.Value] interface.
func (e *logLevelStr) Set(v string) error {
	switch v {
	case launchr.LogLevelStrDisabled, launchr.LogLevelStrDebug, launchr.LogLevelStrInfo, launchr.LogLevelStrWarn, launchr.LogLevelStrError:
		*e = logLevelStr(v)
		return nil
	default:
		return fmt.Errorf(
			`must be one of %s, %s, %s, %s, %s`,
			launchr.LogLevelStrDisabled, launchr.LogLevelStrDebug, launchr.LogLevelStrInfo, launchr.LogLevelStrWarn, launchr.LogLevelStrError,
		)
	}
}

// Type implements [github.com/spf13/pflag.Value] interface.
func (e *logLevelStr) Type() string {
	return "logLevelStr"
}

// OnAppInit implements [launchr.OnAppInitPlugin] interface.
func (p Plugin) OnAppInit(app launchr.App) error {
	verbosity := 0
	quiet := false
	var logFormat LogFormat
	var logLvlStr logLevelStr

	// Assert we are able to access internal functionality.
	appInternal, ok := app.(launchr.AppInternal)
	if !ok {
		return nil
	}
	// Define verbosity flags.
	cmd := appInternal.RootCmd()
	pflags := cmd.PersistentFlags()
	// Make sure not to fail on unknown flags because we are parsing early.
	unkFlagsBkp := pflags.ParseErrorsWhitelist.UnknownFlags
	pflags.ParseErrorsWhitelist.UnknownFlags = true
	pflags.CountVarP(&verbosity, "verbose", "v", "log verbosity level, use -vvvv DEBUG, -vvv INFO, -vv WARN, -v ERROR")
	pflags.VarP(&logLvlStr, "log-level", "", "log level, same as -v, can be: DEBUG, INFO, WARN, ERROR or NONE (default NONE)")
	pflags.VarP(&logFormat, "log-format", "", "log format, can be: pretty, plain or json (default pretty)")
	pflags.BoolVarP(&quiet, "quiet", "q", false, "disable output to the console")

	// Parse available flags.
	err := pflags.Parse(appInternal.CmdEarlyParsed().Args)
	if launchr.IsCommandErrHelp(err) {
		return nil
	}
	if err != nil {
		// It shouldn't happen here.
		panic(err)
	}
	pflags.ParseErrorsWhitelist.UnknownFlags = unkFlagsBkp

	// Set quiet mode.
	launchr.Term().EnableOutput()
	if !quiet && launchr.EnvVarQuietMode.Get() == "1" {
		quiet = true
	}
	if quiet {
		_ = launchr.EnvVarQuietMode.Set("1")
		launchr.Term().DisableOutput()
		app.SetStreams(launchr.NoopStreams())
	}

	// Select log level based on priority of definition.
	logLevel := launchr.LogLevelDisabled
	if pflags.Changed("log-level") {
		logLevel = launchr.LogLevelFromString(string(logLvlStr))
	} else if pflags.Changed("verbose") {
		logLevel = logLevelFlagInt(verbosity)
	} else if logLvlEnv := launchr.EnvVarLogLevel.Get(); logLvlEnv != "" {
		logLevel = launchr.LogLevelFromString(logLvlEnv)
	}

	streams := app.Streams()
	out := streams.Out()
	// Set terminal output.
	launchr.Term().SetOutput(out)

	logger := NewLogger(logFormat, logLevel, out)
	launchr.SetLogger(logger)

	if logLevel != launchr.LogLevelDisabled {
		_ = launchr.EnvVarLogLevel.Set(logLevel.String())
		_ = launchr.EnvVarLogFormat.Set(logFormat.String())
	}

	cmd.SetOut(out)
	cmd.SetErr(streams.Err())

	var am action.Manager
	app.GetService(&am)
	globalFlags := p.getPersistentFlagsDefinition()

	persistentFlags := am.GetPersistentFlags()
	persistentFlags.AddDefinitions(globalFlags)
	for _, f := range globalFlags {
		name := f.Name
		var value any

		switch f.Name {
		case "log-level":
			value = logger.Level().String()
		case "log-format":
			value = logFormat.String()
		case "quiet":
			value = quiet
		}

		persistentFlags.Set(name, value)
	}
	am.AddDecorators(WithCustomLogger)

	return nil
}

func NewLogger(logFormat LogFormat, logLevel launchr.LogLevel, out *launchr.Out) *launchr.Logger {
	var logger *launchr.Logger
	if logLevel == launchr.LogLevelDisabled {
		logger = launchr.NewTextHandlerLogger(io.Discard)
	} else {
		switch logFormat {
		case LogFormatPlain:
			logger = launchr.NewTextHandlerLogger(out)
		case LogFormatJSON:
			logger = launchr.NewJSONHandlerLogger(out)
		default:
			logger = launchr.NewConsoleLogger(out)
		}
	}

	launchr.Term().Printfln("NewLogger")

	logger.SetLevel(logLevel)

	return logger
}

// WithCustomLogger adds a default [Runtime] for an action.
func WithCustomLogger(_ action.Manager, a *action.Action) {
	if a.Runtime() == nil {
		return
	}

	if env, ok := a.Runtime().(action.RuntimeLoggerAware); ok {
		var logFormat LogFormat
		if lfStr, ok := a.Input().PersistentFlag("log-format").(string); ok {
			logFormat = LogFormat(lfStr)
		}

		var logLevel launchr.LogLevel
		if llStr, ok := a.Input().PersistentFlag("log-level").(string); ok {
			logLevel = launchr.LogLevelFromString(llStr)
		}

		logger := NewLogger(logFormat, logLevel, a.Input().Streams().Out())
		env.SetLogger(logger)
	}
}

func (p Plugin) getPersistentFlagsDefinition() action.ParametersList {
	return action.ParametersList{
		action.NewDefParameter(action.DefParameter{
			Name:    "log-level",
			Title:   "log-level",
			Type:    jsonschema.String,
			Default: "NONE",
			Enum:    []any{"DEBUG", "INFO", "WARN", "ERROR", "NONE"},
		}),
		action.NewDefParameter(action.DefParameter{
			Name:    "log-format",
			Title:   "log-format",
			Type:    jsonschema.String,
			Default: "pretty",
			Enum:    []any{"pretty", "plain", "json"},
		}),
		action.NewDefParameter(action.DefParameter{
			Name:        "quiet",
			Title:       "quiet",
			Description: "disable output to the console",
			Type:        jsonschema.Boolean,
			Default:     false,
		}),
	}
}

func logLevelFlagInt(v int) launchr.LogLevel {
	switch v {
	case 0:
		return launchr.LogLevelDisabled
	case 1:
		return launchr.LogLevelError
	case 2:
		return launchr.LogLevelWarn
	case 3:
		return launchr.LogLevelInfo
	case 4:
		return launchr.LogLevelDebug
	default:
		return launchr.LogLevelDisabled
	}
}
