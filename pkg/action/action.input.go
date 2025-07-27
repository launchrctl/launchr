package action

import (
	"maps"
	"reflect"
	"strings"

	"github.com/launchrctl/launchr/internal/launchr"
	"github.com/launchrctl/launchr/pkg/jsonschema"
)

// inputMapKeyArgsPos is a special map key to store positional arguments.
const inputMapKeyArgsPos = "__positional_strings"

type (
	// InputParams is a type alias for action arguments/options.
	InputParams = map[string]any
)

// Input is a container for action input arguments and options.
type Input struct {
	action    *Action
	validated bool

	// args contains parsed and named arguments.
	args InputParams
	// opts contains parsed options with default values.
	opts InputParams
	// groups contains input parameters grouped by unique name
	groups map[string]InputParams
	// io contains out/in/err destinations.
	io launchr.Streams

	// argsPos contains raw positional arguments.
	argsPos []string
	// argsRaw contains arguments that were input by a user and without default values.
	argsRaw InputParams
	// optsRaw contains options that were input by a user and without default values.
	optsRaw InputParams
}

// NewInput creates new input with named Arguments args and named Options opts.
// Options are filled with default values.
func NewInput(a *Action, args InputParams, opts InputParams, io launchr.Streams) *Input {
	def := a.ActionDef()
	// Process positional first.
	argsPos := argsNamedToPos(args, def.Arguments)
	// Make sure the special key doesn't leak.
	delete(args, inputMapKeyArgsPos)
	return &Input{
		action:  a,
		args:    setParamDefaults(args, def.Arguments),
		argsRaw: args,
		argsPos: argsPos,
		opts:    setParamDefaults(opts, def.Options),
		optsRaw: opts,
		groups:  make(map[string]InputParams),
		io:      io,
	}
}

// ArgsPosToNamed creates named arguments input.
func ArgsPosToNamed(a *Action, args []string) (InputParams, error) {
	def := a.ActionDef()
	mapped := make(InputParams, len(args))
	for i, arg := range args {
		if i < len(def.Arguments) {
			var err error
			mapped[def.Arguments[i].Name], err = castArgStrToType(arg, def.Arguments[i])
			if err != nil {
				return nil, err
			}
		}
	}
	// Store a special key to have positional arguments as []string in [NewInput].
	mapped[inputMapKeyArgsPos] = args
	return mapped, nil
}

func castArgStrToType(v string, pdef *DefParameter) (any, error) {
	var err error
	if pdef.Type != jsonschema.Array {
		return jsonschema.ConvertStringToType(v, pdef.Type)
	}
	items := strings.Split(v, ",")
	res := make([]any, len(items))
	for i, item := range items {
		res[i], err = jsonschema.ConvertStringToType(item, pdef.Items.Type)
		if err != nil {
			return nil, err
		}
	}
	return res, nil
}

// IsValidated returns input status.
func (input *Input) IsValidated() bool {
	return input.validated
}

// SetValidated marks input as validated.
func (input *Input) SetValidated(v bool) {
	input.validated = v
}

// Arg returns argument by a name.
func (input *Input) Arg(name string) any {
	return input.Args()[name]
}

// SetArg sets an argument value.
func (input *Input) SetArg(name string, val any) {
	input.argsRaw[name] = val
	input.args[name] = val
}

// UnsetArg unsets the arguments and recalculates default and positional values.
func (input *Input) UnsetArg(name string) {
	delete(input.args, name)
	delete(input.argsRaw, name)
	input.args = setParamDefaults(input.argsRaw, input.action.ActionDef().Arguments)
	input.argsPos = argsNamedToPos(input.argsRaw, input.action.ActionDef().Arguments)
}

// IsArgChanged checks if an argument was changed by user.
func (input *Input) IsArgChanged(name string) bool {
	_, ok := input.argsRaw[name]
	return ok
}

// Opt returns option by a name.
func (input *Input) Opt(name string) any {
	return input.Opts()[name]
}

// SetOpt sets an option value.
func (input *Input) SetOpt(name string, val any) {
	input.optsRaw[name] = val
	input.opts[name] = val
}

// UnsetOpt unsets the option and recalculates default values.
func (input *Input) UnsetOpt(name string) {
	delete(input.optsRaw, name)
	delete(input.opts, name)
	input.opts = setParamDefaults(input.opts, input.action.ActionDef().Options)
}

// IsOptChanged checks if an option was changed by user.
func (input *Input) IsOptChanged(name string) bool {
	_, ok := input.optsRaw[name]
	return ok
}

// GroupFlags returns stored group flags values.
func (input *Input) GroupFlags(group string) InputParams {
	if gp, ok := input.groups[group]; ok {
		return gp
	}
	return make(InputParams)
}

// GetFlagInGroup returns a group flag by name.
func (input *Input) GetFlagInGroup(group, name string) any {
	return input.GroupFlags(group)[name]
}

// SetFlagInGroup sets group flag value.
func (input *Input) SetFlagInGroup(group, name string, val any) {
	gp, ok := input.groups[group]
	if !ok {
		gp = make(InputParams)
		input.groups[group] = gp
	}
	gp[name] = val
}

// Args returns input named and processed arguments.
func (input *Input) Args() InputParams {
	return input.args
}

// ArgsChanged returns arguments that were set manually by user (not processed).
func (input *Input) ArgsChanged() InputParams {
	return input.argsRaw
}

// ArgsPositional returns positional arguments set by user (not processed).
func (input *Input) ArgsPositional() []string {
	return input.argsPos
}

// Opts returns options with default values and processed.
func (input *Input) Opts() InputParams {
	return input.opts
}

// OptsChanged returns options that were set manually by user (not processed).
func (input *Input) OptsChanged() InputParams {
	return input.optsRaw
}

// Streams returns input io.
func (input *Input) Streams() launchr.Streams {
	return input.io
}

func argsNamedToPos(args InputParams, argsDef ParametersList) []string {
	if args == nil {
		return nil
	}
	if inpArgsPos, ok := args[inputMapKeyArgsPos]; ok {
		return inpArgsPos.([]string)
	}
	res := make([]string, len(argsDef))
	for i := 0; i < len(argsDef); i++ {
		res[i], _ = args[argsDef[i].Name].(string)
	}
	return res
}

func setParamDefaults(params InputParams, paramDef ParametersList) InputParams {
	res := maps.Clone(params)
	if res == nil {
		res = make(InputParams)
	}
	for _, d := range paramDef {
		k := d.Name
		v, ok := params[k]
		// Set default values.
		if ok {
			res[k] = v
		} else if d.Default != nil {
			res[k] = d.Default
		}
		// Cast to []any slice because jsonschema validator supports only this type.
		if d.Type == jsonschema.Array {
			res[k] = CastSliceTypedToAny(res[k])
		}
	}
	return res
}

// CastSliceTypedToAny converts an unknown slice to []any slice.
func CastSliceTypedToAny(slice any) []any {
	if slice == nil {
		return nil
	}
	if slice, okAny := slice.([]any); okAny {
		return slice
	}
	val := reflect.ValueOf(slice)
	if val.Kind() != reflect.Slice {
		return nil
	}
	res := make([]any, val.Len())
	for i := 0; i < val.Len(); i++ {
		res[i] = val.Index(i).Interface()
	}
	return res
}

// CastSliceAnyToTyped converts []any slice to a typed slice.
func CastSliceAnyToTyped[T any](orig []any) []T {
	res := make([]T, len(orig))
	for i := 0; i < len(orig); i++ {
		// Ignore the convert error if the value is not of the specified type.
		res[i], _ = orig[i].(T)
	}
	return res
}

// InputArgSlice is a helper function to get an argument of specific type slice.
func InputArgSlice[T any](input *Input, name string) []T {
	return CastSliceAnyToTyped[T](input.Arg(name).([]any))
}

// InputOptSlice is a helper function to get an option of specific type slice.
func InputOptSlice[T any](input *Input, name string) []T {
	return CastSliceAnyToTyped[T](input.Opt(name).([]any))
}
