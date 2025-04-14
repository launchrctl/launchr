// Package verbosity is a plugin of launchr to configure log level of the app.
package verbosity

import (
	"fmt"
	"math"

	"github.com/launchrctl/launchr/internal/launchr"
	"github.com/launchrctl/launchr/pkg/action"
	"github.com/launchrctl/launchr/pkg/jsonschema"
	"github.com/launchrctl/launchr/plugins/actionscobra"

	"github.com/gookit/event"
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

	var am action.Manager
	app.GetService(&am)
	am.AddGlobalsDef(p.getGlobals())

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
	// Enable logger.
	if logLevel != launchr.LogLevelDisabled {
		if logFormat == "" && launchr.EnvVarLogFormat.Get() != "" {
			logFormat = LogFormat(launchr.EnvVarLogFormat.Get())
		}
		var logger *launchr.Logger
		switch logFormat {
		case LogFormatPlain:
			logger = launchr.NewTextHandlerLogger(out)
		case LogFormatJSON:
			logger = launchr.NewJSONHandlerLogger(out)
		default:
			logger = launchr.NewConsoleLogger(out)
		}
		launchr.SetLogger(logger)
		// Save env variable for subprocesses.
		_ = launchr.EnvVarLogLevel.Set(logLevel.String())
		_ = launchr.EnvVarLogFormat.Set(logFormat.String())
	}
	launchr.Log().SetLevel(logLevel)
	cmd.SetOut(out)
	cmd.SetErr(streams.Err())

	p.addEvents(logLevel, logFormat, quiet)

	return nil
}

func (p Plugin) addEvents(logLevel launchr.LogLevel, logFormat LogFormat, quiet bool) {
	ed := launchr.EventDispatcher()
	ed.On(actionscobra.EventCobraPreRun, event.ListenerFunc(func(e event.Event) error {
		a := e.Get("input")
		if inputObj, okObj := a.(*action.Input); okObj {
			if logFormat != "" {
				inputObj.SetGlobalOpt("log-format", logFormat.String())
			}

			inputObj.SetGlobalOpt("log-level", logLevel.String())
			inputObj.SetGlobalOpt("quiet", quiet)
		}

		return nil
	}), event.Normal)

	ed.On(action.EventActionPreExecute, event.ListenerFunc(func(e event.Event) error {
		a := e.Get("action")
		if actionObj, okObj := a.(*action.Action); okObj {
			input := actionObj.Input()
			ll := input.Globals()["log-level"]
			lf := input.Globals()["log-format"]
			str := actionObj.Input().Streams()

			if lf == "" && launchr.EnvVarLogFormat.Get() != "" {
				lf = LogFormat(launchr.EnvVarLogFormat.Get())
			}

			var logger *launchr.Logger
			switch lf {
			case LogFormatPlain:
				logger = launchr.NewTextHandlerLogger(str.Out())
			case LogFormatJSON:
				logger = launchr.NewJSONHandlerLogger(str.Out())
			default:
				logger = launchr.NewConsoleLogger(str.Out())
			}

			if ll == launchr.LogLevelStrDisabled {
				logger.SetOutput(launchr.NoopStreams().Out())
			}

			actionObj.Runtime().SetLogger(logger)
		}

		return nil
	}), event.Normal)
}

func (p Plugin) getGlobals() action.ParametersList {
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
