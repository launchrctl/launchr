package action

import (
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/launchrctl/launchr/pkg/jsonschema"
)

// ValueProcessor defines an interface for processing a value based on its type and some options.
type ValueProcessor interface {
	IsApplicable(valueType jsonschema.Type) bool
	OptionsType() ValueProcessorOptions
	Handler(opts ValueProcessorOptions) ValueProcessorHandler
}

// ValueProcessorContext is related context data for ValueProcessor.
type ValueProcessorContext struct {
	ValOrig   any           // ValOrig is the value before processing.
	IsChanged bool          // IsChanged indicates if the value was input by user.
	DefParam  *DefParameter // DefParam is the definition of the currently processed parameter.
	Action    *Action       // Action is the related action definition.
}

// ValueProcessorHandler is an actual implementation of [ValueProcessor] that processes the incoming value.
type ValueProcessorHandler func(v any, ctx ValueProcessorContext) (any, error)

// ValueProcessorOptions is a common type for value processor options
type ValueProcessorOptions interface {
	Validate() error
}

// ValueProcessorOptionsFields provides option fields that must be decoded.
type ValueProcessorOptionsFields interface {
	DecodeFields() any
}

// GenericValueProcessorOptions is a common [ValueProcessorOptions] with validation.
type GenericValueProcessorOptions[T any] struct {
	Fields T
}

// DecodeFields implements [ValueProcessorOptionsFields] interface.
func (o *GenericValueProcessorOptions[T]) DecodeFields() any {
	return &o.Fields
}

// Validate implements [ValueProcessorOptions] interface.
func (o *GenericValueProcessorOptions[T]) Validate() error {
	val := reflect.ValueOf(o.Fields)
	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)

		for _, fnName := range tagValidateDef(fieldType) {
			switch fnName {
			case "not-empty":
				if field.IsZero() {
					return ErrValueProcessorOptionsFieldValidation{
						Field:  tagYamlOrStructName(fieldType),
						Reason: "required",
					}
				}
			}
		}
	}

	return nil
}

func tagYamlOrStructName(ftype reflect.StructField) string {
	tag := ftype.Tag.Get("yaml")
	if tag == "" {
		return ftype.Name
	}
	fields := strings.Split(tag, ",")
	return fields[0]
}

func tagValidateDef(ftype reflect.StructField) []string {
	validations := ftype.Tag.Get("validate")
	return strings.Split(validations, " ")
}

// ValueProcessorOptionsEmpty when [ValueProcessor] doesn't have options.
type ValueProcessorOptionsEmpty = *GenericValueProcessorOptions[struct{}]

// GenericValueProcessor is a common [ValueProcessor].
type GenericValueProcessor[T ValueProcessorOptions] struct {
	Types []jsonschema.Type
	Fn    GenericValueProcessorHandler[T]
}

// GenericValueProcessorHandler is an extension of [ValueProcessorHandler] to have typed [ValueProcessorOptions].
type GenericValueProcessorHandler[T ValueProcessorOptions] func(v any, opts T, ctx ValueProcessorContext) (any, error)

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
	return t
}

// Handler implements [ValueProcessor] interface.
func (p GenericValueProcessor[T]) Handler(opts ValueProcessorOptions) ValueProcessorHandler {
	optsT, ok := opts.(T)
	if !ok {
		panic(fmt.Sprintf("incorrect options type, expected %T, actual %T, please ensure the code integrity", optsT, opts))
	}
	return func(v any, ctx ValueProcessorContext) (any, error) {
		return p.Fn(v, optsT, ctx)
	}
}

func initValueProcessors(list map[string]ValueProcessor, p *DefParameter) ([]ValueProcessorHandler, error) {
	var err error
	processors := make([]ValueProcessorHandler, 0, len(p.Process))
	for _, procDef := range p.Process {
		proc, ok := list[procDef.ID]
		if !ok {
			return nil, ErrValueProcessorNotExist(procDef.ID)
		}

		// Check type is supported by a processor.
		if !proc.IsApplicable(p.Type) {
			return nil, ErrValueProcessorNotApplicable{
				Type:      p.Type,
				Processor: procDef.ID,
			}
		}

		// Parse value processor options.
		opts := proc.OptionsType()
		if procDef.optsRaw != nil {
			if optsFields, ok := opts.(ValueProcessorOptionsFields); ok {
				err = procDef.optsRaw.Decode(optsFields.DecodeFields())
			} else {
				err = procDef.optsRaw.Decode(opts)
			}
			if err != nil {
				return nil, err
			}
		}

		// Validate the options.
		if err = opts.Validate(); err != nil {
			return nil, ErrValueProcessorOptionsValidation{Processor: procDef.ID, Err: err}
		}

		// Add to processors.
		processors = append(processors, proc.Handler(opts))
	}
	return processors, nil
}
