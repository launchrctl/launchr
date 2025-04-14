package actionscobra

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/launchrctl/launchr/internal/launchr"
	"github.com/launchrctl/launchr/pkg/action"
	"github.com/launchrctl/launchr/pkg/jsonschema"
)

// CobraImpl returns cobra command implementation for an action command.
func CobraImpl(a *action.Action, streams launchr.Streams, manager action.Manager) (*launchr.Command, error) {
	def := a.ActionDef()
	options := make(action.InputParams)
	runOpts := make(action.InputParams)
	cmd := &launchr.Command{
		Use:     getCmdUse(a),
		Short:   getDesc(def.Title, def.Description),
		Aliases: def.Aliases,
		PreRunE: func(cmd *launchr.Command, args []string) error {
			// Set action input.
			argsNamed, err := action.ArgsPosToNamed(a, args)
			if err != nil {
				return err
			}
			optsChanged := derefOpts(filterChangedFlags(cmd, options))
			input := action.NewInput(a, argsNamed, optsChanged, streams)

			// Store runtime flags in the input.
			if rt, ok := a.Runtime().(action.RuntimeFlags); ok {
				runtimeFlagsGroup := rt.GetFlags()
				runOpts = derefOpts(filterChangedFlags(cmd, runOpts))
				for k, v := range runOpts {
					input.SetFlagInGroup(runtimeFlagsGroup.GetName(), k, v)
				}
			}

			// Retrieve the current persistent flags state and pass to action. It will be later used during decorating
			// or other action steps.
			// Flags are immutable in action.
			persistentFlagsGroup := manager.GetPersistentFlags()
			for k, v := range persistentFlagsGroup.GetAll() {
				input.SetFlagInGroup(persistentFlagsGroup.GetName(), k, v)
			}

			// Validate input before setting to action.
			if err = manager.ValidateInput(a, input); err != nil {
				return err
			}

			// Set input.
			if err = a.SetInput(input); err != nil {
				return err
			}

			// Re-apply all registered decorators to action before its execution.
			// Triggered after action.SetInput to ensure decorators have access to all necessary data from the input
			// to proceed.
			manager.Decorate(a)

			return nil
		},
		RunE: func(cmd *launchr.Command, _ []string) (err error) {
			// Don't show usage help on a runtime error.
			cmd.SilenceUsage = true

			_, err = manager.Run(cmd.Context(), a)
			return err
		},
	}

	// Collect action flags.
	err := setCmdFlags(cmd.Flags(), def.Options, options)
	if err != nil {
		return nil, err
	}

	if rt, ok := a.Runtime().(action.RuntimeFlags); ok {
		runtimeFlagsGroup := rt.GetFlags()
		err = setCmdFlags(cmd.Flags(), runtimeFlagsGroup.GetDefinitions(), runOpts)
		if err != nil {
			return nil, err
		}
	}

	// Update usage template to represent arguments, options and runtime options.
	cmd.SetUsageFunc(usageTplFn(a))

	return cmd, nil
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

func setCmdFlags(flags *pflag.FlagSet, defs action.ParametersList, opts action.InputParams) error {
	for _, opt := range defs {
		v, err := setFlag(flags, opt)
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

func setFlag(flags *pflag.FlagSet, param *action.DefParameter) (any, error) {
	var val any
	desc := getDesc(param.Title, param.Description)
	// Get default value if it's not set.
	dval, err := jsonschema.EnsureType(param.Type, param.Default)
	if err != nil {
		return nil, err
	}
	switch param.Type {
	case jsonschema.String:
		val = flags.StringP(param.Name, param.Shorthand, dval.(string), desc)
	case jsonschema.Integer:
		val = flags.IntP(param.Name, param.Shorthand, dval.(int), desc)
	case jsonschema.Number:
		val = flags.Float64P(param.Name, param.Shorthand, dval.(float64), desc)
	case jsonschema.Boolean:
		val = flags.BoolP(param.Name, param.Shorthand, dval.(bool), desc)
	case jsonschema.Array:
		dslice := dval.([]any)
		switch param.Items.Type {
		case jsonschema.String:
			val = flags.StringSliceP(param.Name, param.Shorthand, action.CastSliceAnyToTyped[string](dslice), desc)
		case jsonschema.Integer:
			val = flags.IntSliceP(param.Name, param.Shorthand, action.CastSliceAnyToTyped[int](dslice), desc)
		case jsonschema.Number:
			val = flags.Float64SliceP(param.Name, param.Shorthand, action.CastSliceAnyToTyped[float64](dslice), desc)
		case jsonschema.Boolean:
			val = flags.BoolSliceP(param.Name, param.Shorthand, action.CastSliceAnyToTyped[bool](dslice), desc)
		default:
			// @todo use flags.Var() and define a custom value, jsonschema accepts "any".
			return nil, fmt.Errorf("json schema array type %q is not implemented", param.Items.Type)
		}
	default:
		return nil, fmt.Errorf("json schema type %q is not implemented", param.Type)
	}
	if param.Required {
		_ = cobra.MarkFlagRequired(flags, param.Name)
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
