package action

import (
	"reflect"
	"strings"

	"github.com/launchrctl/launchr/internal/launchr"
	"github.com/launchrctl/launchr/pkg/jsonschema"
)

// inputMapKeyPosArgs is a special map key to store positional arguments.
const inputMapKeyPosArgs = "__pos__"

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
	// io contains out/in/err destinations. @todo should it be in Input?
	io launchr.Streams

	// argsPos contains raw positional arguments.
	argsPos []string
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
	delete(args, inputMapKeyPosArgs)
	return &Input{
		action:  a,
		args:    setParamDefaults(args, def.Arguments),
		argsPos: argsPos,
		opts:    setParamDefaults(opts, def.Options),
		optsRaw: opts,
		io:      io,
	}
}

// ArgsPosToNamed creates named arguments input.
func ArgsPosToNamed(a *Action, args []string) InputParams {
	def := a.ActionDef()
	mapped := make(InputParams, len(args))
	for i, arg := range args {
		if i < len(def.Arguments) {
			mapped[def.Arguments[i].Name] = castArgStrToType(arg, def.Arguments[i])
		}
	}
	// Store a special key
	mapped[inputMapKeyPosArgs] = args
	return mapped
}

func castArgStrToType(v string, pdef *DefParameter) any {
	var err error
	if pdef.Type != jsonschema.Array {
		res, err := jsonschema.ConvertStringToType(v, pdef.Type)
		if err != nil {
			return jsonschema.MustTypeDefault(pdef.Type, nil)
		}
		return res
	}
	items := strings.Split(v, ",")
	res := make([]any, len(items))
	for i, item := range items {
		res[i], err = jsonschema.ConvertStringToType(item, pdef.Items.Type)
		if err != nil {
			return jsonschema.MustTypeDefault(pdef.Items.Type, nil)
		}
	}
	return res
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
	return input.ArgsNamed()[name]
}

// SetArg sets an argument value.
func (input *Input) SetArg(name string, val any) {
	input.optsRaw[name] = val
	input.opts[name] = val
}

// UnsetArg unsets the arguments and recalculates default and positional values.
func (input *Input) UnsetArg(name string) {
	delete(input.args, name)
	input.args = setParamDefaults(input.args, input.action.ActionDef().Arguments)
	input.argsPos = argsNamedToPos(input.args, input.action.ActionDef().Arguments)
}

// IsArgChanged checks if an argument was changed by user.
func (input *Input) IsArgChanged(name string) bool {
	_, ok := input.args[name]
	return ok
}

// Opt returns option by a name.
func (input *Input) Opt(name string) any {
	return input.OptsAll()[name]
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

// ArgsNamed returns input named and processed arguments.
func (input *Input) ArgsNamed() InputParams {
	return input.args
}

// ArgsPositional returns positional arguments set by user (not processed).
func (input *Input) ArgsPositional() []string {
	return input.argsPos
}

// OptsAll returns options with default values and processed.
func (input *Input) OptsAll() InputParams {
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
	if inpArgsPos, ok := args[inputMapKeyPosArgs]; ok {
		return inpArgsPos.([]string)
	}
	res := make([]string, len(argsDef))
	for i := 0; i < len(argsDef); i++ {
		res[i], _ = args[argsDef[i].Name].(string)
	}
	return res
}

func setParamDefaults(params InputParams, paramDef ParametersList) InputParams {
	res := copyMap(params)
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
		res[i] = orig[i].(T)
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
