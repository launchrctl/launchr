package actionscobra

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/spf13/pflag"

	"github.com/launchrctl/launchr/internal/launchr"
	"github.com/launchrctl/launchr/pkg/action"
	"github.com/launchrctl/launchr/pkg/jsonschema"
)

// CobraImpl returns cobra command implementation for an action command.
func CobraImpl(a *action.Action, streams launchr.Streams) (*launchr.Command, error) {
	def, err := a.Raw()
	if err != nil {
		return nil, err
	}
	actConf := def.Action
	argsDef := actConf.Arguments
	use := a.ID
	for _, p := range argsDef {
		use += " " + p.Name
	}
	options := make(action.InputParams)
	runOpts := make(action.InputParams)
	cmd := &launchr.Command{
		Use: use,
		// @todo: maybe we need a long template for arguments description
		// @todo: have aliases documented in help
		Short:   getDesc(actConf.Title, actConf.Description),
		Aliases: actConf.Aliases,
		RunE: func(cmd *launchr.Command, args []string) error {
			// Don't show usage help on a runtime error.
			cmd.SilenceUsage = true

			err = a.EnsureLoaded()
			if err != nil {
				return err
			}

			// Set action input.
			argsNamed, errPos := action.ArgsPosToNamed(a, args)
			if errPos != nil {
				return errPos
			}
			optsChanged := derefOpts(filterChangedFlags(cmd, options))
			input := action.NewInput(a, argsNamed, optsChanged, streams)
			// Pass to the runtime its flags.
			if r, ok := a.Runtime().(action.RuntimeFlags); ok {
				runOpts = derefOpts(filterChangedFlags(cmd, runOpts))
				err = r.UseFlags(runOpts)
				if err != nil {
					return err
				}
				if err = r.ValidateInput(a, input); err != nil {
					return err
				}
			}

			// Set and validate input.
			if err = a.SetInput(input); err != nil {
				return err
			}

			// @todo can we use action manager here and Manager.Run()
			return a.Execute(cmd.Context())
		},
	}

	// Collect action flags.
	err = setCommandOptions(cmd, actConf.Options, options)
	if err != nil {
		return nil, err
	}
	// Collect runtime flags.
	globalFlags := []string{"help"}

	if env, ok := a.Runtime().(action.RuntimeFlags); ok {
		err = setCommandOptions(cmd, env.FlagsDefinition(), runOpts)
		if err != nil {
			return nil, err
		}

		for _, opt := range env.FlagsDefinition() {
			globalFlags = append(globalFlags, opt.Name)
		}
	}

	// Update usage template according new global flags
	updateUsageTemplate(cmd, globalFlags)

	return cmd, nil
}

func updateUsageTemplate(cmd *launchr.Command, globalOpts []string) {
	cmd.InitDefaultHelpFlag()
	originalFlags := cmd.LocalFlags()
	if !originalFlags.HasAvailableFlags() {
		return
	}

	localFlags := pflag.NewFlagSet("local", pflag.ContinueOnError)
	globalFlags := pflag.NewFlagSet("global", pflag.ContinueOnError)

	originalFlags.VisitAll(func(flag *pflag.Flag) {
		toAdd := false
		for _, name := range globalOpts {
			if flag.Name == name {
				toAdd = true
				break
			}
		}

		if toAdd {
			globalFlags.AddFlag(flag)
		} else {
			localFlags.AddFlag(flag)
		}
	})

	usagesLocal := strings.TrimRight(localFlags.FlagUsages(), " ")
	usagesGlobal := strings.TrimRight(globalFlags.FlagUsages(), " ")

	cmd.SetUsageTemplate(fmt.Sprintf(getUsageTemplate(), usagesLocal, usagesGlobal))
}

func getUsageTemplate() string {
	return `Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

Available Commands:{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{else}}{{range $group := .Groups}}

{{.Title}}{{range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if not .AllChildCommandsHaveGroup}}

Additional Commands:{{range $cmds}}{{if (and (eq .GroupID "") (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
%s
Global Action Flags:
%s
Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`
}

func filterChangedFlags(cmd *launchr.Command, opts action.InputParams) action.InputParams {
	filtered := make(action.InputParams)
	for name, flag := range opts {
		// Filter options not set.
		if opts[name] != nil && cmd.Flags().Changed(name) {
			filtered[name] = flag
		}
	}
	return filtered
}

func setCommandOptions(cmd *launchr.Command, defs action.ParametersList, opts action.InputParams) error {
	for _, opt := range defs {
		v, err := setFlag(cmd, opt)
		if err != nil {
			return err
		}
		opts[opt.Name] = v
	}
	return nil
}

func getDesc(title string, desc string) string {
	parts := make([]string, 0, 2)
	if title != "" {
		parts = append(parts, title)
	}
	if desc != "" {
		parts = append(parts, desc)
	}
	return strings.Join(parts, ": ")
}

func setFlag(cmd *launchr.Command, opt *action.DefParameter) (any, error) {
	var val any
	desc := getDesc(opt.Title, opt.Description)
	// Get default value if it's not set.
	dval, err := jsonschema.EnsureType(opt.Type, opt.Default)
	if err != nil {
		return nil, err
	}
	switch opt.Type {
	case jsonschema.String:
		val = cmd.Flags().StringP(opt.Name, opt.Shorthand, dval.(string), desc)
	case jsonschema.Integer:
		val = cmd.Flags().IntP(opt.Name, opt.Shorthand, dval.(int), desc)
	case jsonschema.Number:
		val = cmd.Flags().Float64P(opt.Name, opt.Shorthand, dval.(float64), desc)
	case jsonschema.Boolean:
		val = cmd.Flags().BoolP(opt.Name, opt.Shorthand, dval.(bool), desc)
	case jsonschema.Array:
		dslice := dval.([]any)
		switch opt.Items.Type {
		case jsonschema.String:
			val = cmd.Flags().StringSliceP(opt.Name, opt.Shorthand, action.CastSliceAnyToTyped[string](dslice), desc)
		case jsonschema.Integer:
			val = cmd.Flags().IntSliceP(opt.Name, opt.Shorthand, action.CastSliceAnyToTyped[int](dslice), desc)
		case jsonschema.Number:
			val = cmd.Flags().Float64SliceP(opt.Name, opt.Shorthand, action.CastSliceAnyToTyped[float64](dslice), desc)
		case jsonschema.Boolean:
			val = cmd.Flags().BoolSliceP(opt.Name, opt.Shorthand, action.CastSliceAnyToTyped[bool](dslice), desc)
		default:
			// @todo use cmd.Flags().Var() and define a custom value, jsonschema accepts "any".
			return nil, fmt.Errorf("json schema array type %q is not implemented", opt.Items.Type)
		}
	default:
		return nil, fmt.Errorf("json schema type %q is not implemented", opt.Type)
	}
	if opt.Required {
		_ = cmd.MarkFlagRequired(opt.Name)
	}
	return val, nil
}

func derefOpts(opts action.InputParams) action.InputParams {
	der := make(action.InputParams, len(opts))
	for k, v := range opts {
		der[k] = derefOpt(v)
	}
	return der
}

func derefOpt(v any) any {
	switch v := v.(type) {
	case *string:
		return *v
	case *bool:
		return *v
	case *int:
		return *v
	case *float64:
		return *v
	case *[]any:
		return *v
	case *[]string:
		return *v
	case *[]int:
		return *v
	case *[]bool:
		return *v
	default:
		if reflect.ValueOf(v).Kind() == reflect.Ptr {
			panic(fmt.Sprintf("error on a value dereferencing: unsupported %T", v))
		}
		return v
	}
}
