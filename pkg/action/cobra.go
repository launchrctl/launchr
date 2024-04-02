package action

import (
	"fmt"
	"reflect"
	"strings"

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
		Use:  use,
		Args: cobra.ExactArgs(len(argsDef)),
		// @todo: maybe we need a long template for arguments description
		Short: getDesc(actConf.Title, actConf.Description),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true // Don't show usage help on a runtime error.
			// Pass to the run environment its flags.
			if env, ok := a.env.(RunEnvironmentFlags); ok {
				err := env.UseFlags(derefOpts(runOpts))
				if err != nil {
					return err
				}
			}

			// Set action input.
			err := a.SetInput(Input{
				Args: argsToMap(args, argsDef),
				Opts: derefOpts(options),
				IO:   streams,
			})
			if err != nil {
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
	if env, ok := a.env.(RunEnvironmentFlags); ok {
		err = setCobraOptions(cmd, env.FlagsDefinition(), runOpts)
		if err != nil {
			return nil, err
		}
	}

	return cmd, nil
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
