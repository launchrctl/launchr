package action

import (
	"reflect"
	"strings"

	"github.com/spf13/cobra"

	"github.com/launchrctl/launchr/pkg/cli"
	"github.com/launchrctl/launchr/pkg/jsonschema"
	"github.com/launchrctl/launchr/pkg/log"
)

// CobraImpl returns cobra command implementation for an action command.
func CobraImpl(a *Action, streams cli.Streams) *cobra.Command {
	actConf := a.ActionDef()
	argsDef := actConf.Arguments
	use := a.ID
	for _, p := range argsDef {
		use += " " + p.Name
	}
	options := make(TypeOpts)
	cmd := &cobra.Command{
		Use:  use,
		Args: cobra.ExactArgs(len(argsDef)),
		// @todo: maybe we need a long template for arguments description
		Short: getDesc(actConf.Title, actConf.Description),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true // Don't show usage help on a runtime error.
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

	for _, opt := range actConf.Options {
		options[opt.Name] = setFlag(cmd, opt)
	}

	return cmd
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

func setFlag(cmd *cobra.Command, opt *Option) interface{} {
	var val interface{}
	desc := getDesc(opt.Title, opt.Description)
	switch opt.Type {
	case jsonschema.String:
		val = cmd.Flags().String(opt.Name, opt.Default.(string), desc)
	case jsonschema.Integer:
		val = cmd.Flags().Int(opt.Name, opt.Default.(int), desc)
	case jsonschema.Number:
		val = cmd.Flags().Float64(opt.Name, opt.Default.(float64), desc)
	case jsonschema.Boolean:
		val = cmd.Flags().Bool(opt.Name, opt.Default.(bool), desc)
	case jsonschema.Array:
		// @todo parse results to requested type somehow
		val = cmd.Flags().StringSlice(opt.Name, opt.Default.([]string), desc)
	default:
		log.Panic("json schema type %s is not implemented", opt.Type)
	}
	if opt.Required {
		_ = cmd.MarkFlagRequired(opt.Name)
	}
	return val
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
		return *v
	default:
		if reflect.ValueOf(v).Kind() == reflect.Ptr {
			log.Panic("error on a value dereferencing: unsupported %T", v)
		}
		return v
	}
}
