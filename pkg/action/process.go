package action

import (
	"fmt"
	"reflect"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/launchrctl/launchr/pkg/jsonschema"
)

// ValueProcessor defines an interface for processing a value based on its type and some options.
type ValueProcessor interface {
	IsApplicable(valueType jsonschema.Type) bool
	OptionsType() ValueProcessorOptions
	Handler(opts ValueProcessorOptions) ValueProcessorFn
}

// ValueProcessorContext is related context data for ValueProcessor.
type ValueProcessorContext struct {
	ValOrig   any                   // ValOrig is the value before processing.
	IsChanged bool                  // IsChanged indicates if the value was input by user.
	Options   ValueProcessorOptions // Options is the [ValueProcessor] configuration.
	DefParam  *DefParameter         // DefParam is the definition of the currently processed parameter.
	Action    *Action               // Action is the related action definition.
}

// ValueProcessorFn is a function signature used as a callback in processors.
type ValueProcessorFn func(v any, ctx ValueProcessorContext) (any, error)

// ValueProcessorOptions is a common type for value processor options
type ValueProcessorOptions interface {
	Validate() error
}

// ValueProcessorOptionsEmpty when [ValueProcessor] doesn't have options.
type ValueProcessorOptionsEmpty struct{}

// Validate implements [ValueProcessorOptions] interface.
func (o *ValueProcessorOptionsEmpty) Validate() error {
	return nil
}

// GenericValueProcessor is a common [ValueProcessor].
type GenericValueProcessor[T ValueProcessorOptions] struct {
	Types []jsonschema.Type
	Fn    GenericValueProcessorFn[T]
}

// GenericValueProcessorFn is an extension of [ValueProcessorFn] to have typed [ValueProcessorOptions].
type GenericValueProcessorFn[T ValueProcessorOptions] func(v any, opts T, ctx ValueProcessorContext) (any, error)

// IsApplicable implements [ValueProcessor] interface.
func (p GenericValueProcessor[T]) IsApplicable(t jsonschema.Type) bool {
	if p.Types == nil {
		// Allow any type.
		return true
	}
	return slices.Contains(p.Types, t)
}

// OptionsType implements [ValueProcessor] interface.
func (p GenericValueProcessor[T]) OptionsType() ValueProcessorOptions {
	var t T
	// Create a new instance of type.
	rtype := reflect.TypeOf(t)
	if rtype.Kind() == reflect.Ptr {
		return reflect.New(rtype.Elem()).Interface().(T)
	}
	panic(fmt.Sprintf("type %T does not implement ValueProcessorOptions correctly: its method(s) must use a pointer receiver (*%T).", t, t))
}

// Handler implements [ValueProcessor] interface.
func (p GenericValueProcessor[T]) Handler(opts ValueProcessorOptions) ValueProcessorFn {
	optsT, ok := opts.(T)
	if !ok {
		panic(fmt.Sprintf("incorrect options type, expected %T, actual %T, please ensure the code integrity", optsT, opts))
	}
	return func(v any, ctx ValueProcessorContext) (any, error) {
		return p.Fn(v, optsT, ctx)
	}
}

// TestCaseValueProcessor is a common test case behavior for [ValueProcessor].
type TestCaseValueProcessor struct {
	Name    string
	Yaml    string
	ErrInit error
	ErrProc error
	Args    InputParams
	Opts    InputParams
	ExpArgs InputParams
	ExpOpts InputParams
}

// Test runs the test for [ValueProcessor].
func (tt TestCaseValueProcessor) Test(t *testing.T, am Manager) {
	a := NewFromYAML(tt.Name, []byte(tt.Yaml))
	// Init processors in the action.
	err := a.SetProcessors(am.GetValueProcessors())
	require.Equal(t, err, tt.ErrInit)
	if tt.ErrInit != nil {
		return
	}
	// Run processors.
	input := NewInput(a, tt.Args, tt.Opts, nil)
	err = a.SetInput(input)
	require.Equal(t, err, tt.ErrProc)
	if tt.ErrProc != nil {
		return
	}
	// Test input is processed.
	input = a.Input()
	if tt.ExpArgs == nil {
		tt.ExpArgs = InputParams{}
	}
	if tt.ExpOpts == nil {
		tt.ExpOpts = InputParams{}
	}
	assert.Equal(t, tt.ExpArgs, input.Args())
	assert.Equal(t, tt.ExpOpts, input.Opts())
}
