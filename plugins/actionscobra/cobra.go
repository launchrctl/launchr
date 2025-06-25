package actionscobra

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/launchrctl/launchr/internal/launchr"
	"github.com/launchrctl/launchr/pkg/action"
	"github.com/launchrctl/launchr/pkg/jsonschema"
)

type objectParseResult struct {
	subFlagHandler *objectSubFlags
	parsedJSON     *jsonObjectValue
}

// GetValue returns the merged result from both JSON and object sub flags
func (r *objectParseResult) GetValue() any {
	// Get values from object sub-flags.
	subFlagValues := r.subFlagHandler.BuildObject()

	// Get JSON values
	var jsonValues map[string]any
	if r.parsedJSON != nil && r.parsedJSON.Value != nil {
		jsonValues = r.parsedJSON.Value
	}

	// If subFlagValues exist, they take precedence
	if len(subFlagValues) > 0 {
		result := make(map[string]any)

		// First add JSON values
		for k, v := range jsonValues {
			result[k] = v
		}

		// Then add/override with subFlag values
		for k, v := range subFlagValues {
			result[k] = v
		}

		return result
	}

	// If there is no subFlagValues, return JSON result
	if len(jsonValues) > 0 {
		return jsonValues
	}

	// return an empty map in case of nothing
	return make(map[string]any)
}

type objectSubFlags struct {
	flagPtrs     map[string]any // Store flag pointers
	flagTypes    map[string]jsonschema.Type
	flagNames    map[string]string // Store actual flag names
	mainFlagName string            // Store the main flag name for change tracking
}

func (h *objectSubFlags) RegisterNestedFlags(flags *pflag.FlagSet, param *action.DefParameter) error {
	h.mainFlagName = param.Name // Store the main flag name
	if param.Properties == nil {
		return nil
	}

	for propName, propDef := range param.Properties {
		dotKey := fmt.Sprintf("%s.%s", param.Name, propName)
		desc := fmt.Sprintf("%s.%s - %s", param.Name, propName, propDef.Description)

		// Store the mapping between the property name and full flag name
		h.flagNames[propName] = dotKey

		// Get default value
		var propDefaultValue any
		if param.Default != nil {
			if defaultMap, ok := param.Default.(map[string]any); ok {
				propDefaultValue = defaultMap[propName]
			}
		}

		dval, err := jsonschema.EnsureType(propDef.Type, propDefaultValue)
		if err != nil {
			return err
		}

		// Register flag and store pointer
		var ptr any
		switch propDef.Type {
		case jsonschema.String:
			ptr = flags.String(dotKey, dval.(string), desc)
		case jsonschema.Integer:
			ptr = flags.Int(dotKey, dval.(int), desc)
		case jsonschema.Number:
			ptr = flags.Float64(dotKey, dval.(float64), desc)
		case jsonschema.Boolean:
			ptr = flags.Bool(dotKey, dval.(bool), desc)
		default:
			//  nested object and arrays aren't supported yet for inline flags.
			continue
		}

		h.flagPtrs[propName] = ptr
		h.flagTypes[propName] = propDef.Type
	}

	return nil
}

// HasChangedFlags checks if any of the related flags have changed
func (h *objectSubFlags) HasChangedFlags(cmd *launchr.Command) bool {
	for _, flagName := range h.flagNames {
		if cmd.Flags().Changed(flagName) {
			return true
		}
	}
	return false
}

func (h *objectSubFlags) BuildObject() map[string]any {
	result := make(map[string]any)

	for propName, flagPtr := range h.flagPtrs {
		flagType := h.flagTypes[propName]

		switch flagType {
		case jsonschema.String:
			if ptr, ok := flagPtr.(*string); ok && *ptr != "" {
				result[propName] = *ptr
			}
		case jsonschema.Integer:
			if ptr, ok := flagPtr.(*int); ok {
				result[propName] = *ptr
			}
		case jsonschema.Number:
			if ptr, ok := flagPtr.(*float64); ok {
				result[propName] = *ptr
			}
		case jsonschema.Boolean:
			if ptr, ok := flagPtr.(*bool); ok {
				result[propName] = *ptr
			}

		default:
			panic(fmt.Sprintf("json schema object prop type %q is not implemented", flagType))
		}
	}

	return result
}

type jsonObjectValue struct {
	Value map[string]any
}

// jsonFlag handles JSON array/object types that aren't natively supported
type jsonFlag struct {
	target *jsonObjectValue
}

// String returns the string representation of the value
func (v *jsonFlag) String() string {
	if v.target == nil || v.target.Value == nil {
		return ""
	}
	jsonBytes, err := json.Marshal(v.target.Value)
	if err != nil {
		return ""
	}
	return string(jsonBytes)
}

// Set parses and sets the value from a string
func (v *jsonFlag) Set(s string) error {
	var parsed map[string]any

	// Try to parse as JSON
	if err := json.Unmarshal([]byte(s), &parsed); err != nil {
		return fmt.Errorf("invalid JSON: %v", err)
	}

	if v.target == nil {
		v.target = &jsonObjectValue{}
	}
	v.target.Value = parsed

	return nil
}

// Type returns the type name for help text
func (v *jsonFlag) Type() string {
	return "json"
}

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
					input.SetFlagInGroup(runtimeFlagsGroup.GetName(), flag, value)
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
		if simpleObj, ok := flag.(*objectParseResult); ok {
			// Check if the main flag OR any of the related sub flags changed
			mainFlagChanged := cmd.Flags().Changed(name)
			nestedFlagsChanged := simpleObj.subFlagHandler.HasChangedFlags(cmd)
			if opts[name] != nil && (mainFlagChanged || nestedFlagsChanged) {
				filtered[name] = flag
			}
		} else {
			// Original logic for non-object flags
			if opts[name] != nil && cmd.Flags().Changed(name) {
				filtered[name] = flag
			}
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
	case jsonschema.Object:
		osf := &objectSubFlags{
			mainFlagName: param.Name,
			flagPtrs:     make(map[string]any),
			flagTypes:    make(map[string]jsonschema.Type),
			flagNames:    make(map[string]string),
		}
		// Register nested flags
		// Currently cover only 1 level of object
		if err = osf.RegisterNestedFlags(flags, param); err != nil {
			return nil, fmt.Errorf("failed to register object nested flags: %w", err)
		}

		jsonTarget := &jsonObjectValue{Value: dval.(map[string]any)}
		// Register JSON flag
		flags.VarP(&jsonFlag{target: jsonTarget}, param.Name, param.Shorthand, desc+" (JSON format)")

		// Return combined result
		val = &objectParseResult{
			parsedJSON:     jsonTarget,
			subFlagHandler: osf,
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
	case *objectParseResult:
		return v.GetValue()
	default:
		if reflect.ValueOf(v).Kind() == reflect.Ptr {
			panic(fmt.Sprintf("error on a value dereferencing: unsupported %T", v))
		}
		return v
	}
}
