package actionscobra

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"

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
				for flag, defaultValue := range runtimeFlagsGroup.GetAll() {
					value := defaultValue
					if runOpts[flag] != nil {
						value = runOpts[flag]
					}
					input.SetFlagInGroup(runtimeFlagsGroup.Name(), flag, value)
				}
			}

			// Retrieve the current persistent flags state and pass to action. It will be later used during decorating
			// or other action steps.
			// Flags are immutable in action.
			persistentFlagsGroup := manager.GetPersistentFlags()
			for k, v := range persistentFlagsGroup.GetAll() {
				input.SetFlagInGroup(persistentFlagsGroup.Name(), k, v)
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

			// Check structured output flags.
			persistentFlags := manager.GetPersistentFlags()
			outputFormat, _ := a.Input().GetFlagInGroup(persistentFlags.Name(), "output").(string)
			structuredOutput := outputFormat == "json" || outputFormat == "yaml"

			// Fail early if structured output is requested but not supported.
			if structuredOutput && a.ActionDef().Result == nil {
				return fmt.Errorf("action %q does not support structured output (no result schema defined)", a.ID)
			}

			// Fail early on unknown output format.
			if outputFormat != "" && !structuredOutput {
				return fmt.Errorf("unknown output format %q, supported: json, yaml", outputFormat)
			}

			// When structured output is requested, silence terminal output
			// so only the encoded result goes to stdout.
			if structuredOutput {
				if rt, ok := a.Runtime().(action.RuntimeTermAware); ok {
					rt.Term().SetOutput(io.Discard)
				}
			}

			ri, err := manager.Run(cmd.Context(), a)

			// Encode structured output.
			if outputFormat == "json" {
				return outputJSON(cmd, ri, err)
			}
			if outputFormat == "yaml" {
				return outputYAML(cmd, ri, err)
			}

			// Text mode: action already printed its human-readable output via term.
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

	// Skip silently incorrect or duplicated shorthands.
	shorthand := param.Shorthand
	if len(shorthand) > 1 {
		launchr.Log().Warn("incorrect shorthand definition for cobra flag, skipping", "parameter", param.Name, "shorthand", shorthand)
		shorthand = ""
	}
	if flags.ShorthandLookup(shorthand) != nil {
		launchr.Log().Warn("duplicate shorthand definition for cobra flag, skipping", "parameter", param.Name, "shorthand", shorthand)
		shorthand = ""
	}
	switch param.Type {
	case jsonschema.String:
		val = flags.StringP(param.Name, shorthand, dval.(string), desc)
	case jsonschema.Integer:
		val = flags.IntP(param.Name, shorthand, dval.(int), desc)
	case jsonschema.Number:
		val = flags.Float64P(param.Name, shorthand, dval.(float64), desc)
	case jsonschema.Boolean:
		val = flags.BoolP(param.Name, shorthand, dval.(bool), desc)
	case jsonschema.Array:
		dslice := dval.([]any)
		switch param.Items.Type {
		case jsonschema.String:
			val = flags.StringSliceP(param.Name, shorthand, action.CastSliceAnyToTyped[string](dslice), desc)
		case jsonschema.Integer:
			val = flags.IntSliceP(param.Name, shorthand, action.CastSliceAnyToTyped[int](dslice), desc)
		case jsonschema.Number:
			val = flags.Float64SliceP(param.Name, shorthand, action.CastSliceAnyToTyped[float64](dslice), desc)
		case jsonschema.Boolean:
			val = flags.BoolSliceP(param.Name, shorthand, action.CastSliceAnyToTyped[bool](dslice), desc)
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
	case *[]float64:
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

// JSONOutput is the structured output format when --json flag is used.
type JSONOutput struct {
	Result any        `json:"result,omitempty"`
	Error  *JSONError `json:"error,omitempty"`
}

// JSONError is the error format for JSON output.
type JSONError struct {
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

// outputJSON handles JSON output for action results.
func outputJSON(cmd *launchr.Command, ri action.RunInfo, execErr error) error {
	out := JSONOutput{}

	if execErr != nil {
		out.Error = &JSONError{Message: execErr.Error()}
		// Include exit code if available.
		var exitErr launchr.ExitError
		if errors.As(execErr, &exitErr) {
			out.Error.Code = fmt.Sprintf("EXIT_%d", exitErr.ExitCode())
		}
	} else {
		out.Result = ri.Result
	}

	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		return fmt.Errorf("failed to encode JSON output: %w", err)
	}

	// Return the original error to preserve exit code.
	return execErr
}

// YAMLOutput is the structured output format when --yaml flag is used.
type YAMLOutput struct {
	Result any        `yaml:"result,omitempty"`
	Error  *YAMLError `yaml:"error,omitempty"`
}

// YAMLError is the error format for YAML output.
type YAMLError struct {
	Message string `yaml:"message"`
	Code    string `yaml:"code,omitempty"`
}

// outputYAML handles YAML output for action results.
func outputYAML(cmd *launchr.Command, ri action.RunInfo, execErr error) error {
	out := YAMLOutput{}

	if execErr != nil {
		out.Error = &YAMLError{Message: execErr.Error()}
		// Include exit code if available.
		var exitErr launchr.ExitError
		if errors.As(execErr, &exitErr) {
			out.Error.Code = fmt.Sprintf("EXIT_%d", exitErr.ExitCode())
		}
	} else {
		out.Result = ri.Result
	}

	enc := yaml.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent(2)
	if err := enc.Encode(out); err != nil {
		return fmt.Errorf("failed to encode YAML output: %w", err)
	}

	// Return the original error to preserve exit code.
	return execErr
}

