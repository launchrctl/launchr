package action

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/spf13/pflag"

	"github.com/spf13/cobra"

	"github.com/launchrctl/launchr/pkg/cli"
	"github.com/launchrctl/launchr/pkg/jsonschema"
	"github.com/launchrctl/launchr/pkg/log"
)

// CobraImpl returns cobra command implementation for an action command.
func CobraImpl(a *Action, streams cli.Streams) (*cobra.Command, error) {
	actConf := a.ActionDef()
	argsDef := actConf.Arguments
	use := a.ID
	for _, p := range argsDef {
		use += " " + p.Name
	}
	options := make(TypeOpts)
	runOpts := make(TypeOpts)
	cmd := &cobra.Command{
		Use: use,
		// Using custom args validation in ValidateInput.
		// @todo: maybe we need a long template for arguments description
		Short:   getDesc(actConf.Title, actConf.Description),
		Aliases: actConf.Aliases,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Pass to the run environment its flags.
			if env, ok := a.env.(RunEnvironmentFlags); ok {
				runOpts = filterFlags(cmd, runOpts)
				err := env.UseFlags(derefOpts(runOpts))
				if err != nil {
					return err
				}
			}

			// Set action input.
			input := Input{
				Args:    argsToMap(args, argsDef),
				Opts:    derefOpts(options),
				IO:      streams,
				ArgsRaw: args,
			}
			if runEnv, ok := a.env.(RunEnvironmentFlags); ok {
				if err := runEnv.ValidateInput(a, input.Args); err != nil {
					return err
				}
			}

			cmd.SilenceUsage = true // Don't show usage help on a runtime error.

			if err := a.SetInput(input); err != nil {
				return err
			}

			// @todo can we use action manager here and Manager.Run()
			return a.Execute(cmd.Context())
		},
	}

	// Collect action flags.
	err := setCobraOptions(cmd, actConf.Options, options)
	if err != nil {
		return nil, err
	}
	// Collect run environment flags.
	globalFlags := []string{"help"}

	if env, ok := a.env.(RunEnvironmentFlags); ok {
		err = setCobraOptions(cmd, env.FlagsDefinition(), runOpts)
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

func updateUsageTemplate(cmd *cobra.Command, globalOpts []string) {
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

func filterFlags(cmd *cobra.Command, opts TypeOpts) TypeOpts {
	filtered := make(TypeOpts)
	for name, flag := range opts {
		// Filter options not set.
		if opts[name] != nil && cmd.Flags().Changed(name) {
			filtered[name] = flag
		}
	}
	return filtered
}

func setCobraOptions(cmd *cobra.Command, defs OptionsList, opts TypeOpts) error {
	for _, opt := range defs {
		v, err := setFlag(cmd, opt)
		if err != nil {
			return err
		}
		opts[opt.Name] = v
	}
	return nil
}

func argsToMap(args []string, argsDef ArgumentsList) TypeArgs {
	mapped := make(TypeArgs, len(args))
	for i, a := range args {
		if i < len(argsDef) {
			mapped[argsDef[i].Name] = a
		}
	}
	return mapped
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

func setFlag(cmd *cobra.Command, opt *Option) (interface{}, error) {
	var val interface{}
	desc := getDesc(opt.Title, opt.Description)
	switch opt.Type {
	case jsonschema.String:
		val = cmd.Flags().StringP(opt.Name, opt.Shorthand, opt.Default.(string), desc)
	case jsonschema.Integer:
		val = cmd.Flags().IntP(opt.Name, opt.Shorthand, opt.Default.(int), desc)
	case jsonschema.Number:
		val = cmd.Flags().Float64P(opt.Name, opt.Shorthand, opt.Default.(float64), desc)
	case jsonschema.Boolean:
		val = cmd.Flags().BoolP(opt.Name, opt.Shorthand, opt.Default.(bool), desc)
	case jsonschema.Array:
		// @todo use Var and define a custom value, jsonschema accepts interface{}
		val = cmd.Flags().StringSliceP(opt.Name, opt.Shorthand, opt.Default.([]string), desc)
	default:
		return nil, fmt.Errorf("json schema type %q is not implemented", opt.Type)
	}
	if opt.Required {
		_ = cmd.MarkFlagRequired(opt.Name)
	}
	return val, nil
}

func derefOpts(opts TypeOpts) TypeOpts {
	der := make(TypeOpts, len(opts))
	for k, v := range opts {
		der[k] = derefOpt(v)
	}
	return der
}

func derefOpt(v interface{}) interface{} {
	switch v := v.(type) {
	case *string:
		return *v
	case *bool:
		return *v
	case *int:
		return *v
	case *float64:
		return *v
	case *[]string:
		// Cast to a slice of interface because jsonschema validator supports only such arrays.
		toAny := make([]interface{}, len(*v))
		for i := 0; i < len(*v); i++ {
			toAny[i] = (*v)[i]
		}
		return toAny
	default:
		// @todo recheck
		if reflect.ValueOf(v).Kind() == reflect.Ptr {
			log.Panic("error on a value dereferencing: unsupported %T", v)
		}
		return v
	}
}
