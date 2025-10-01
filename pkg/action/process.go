package action

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"text/template"

	"github.com/launchrctl/launchr/internal/launchr"
	"github.com/launchrctl/launchr/pkg/jsonschema"
)

// TemplateFuncContext stores context used for processing.
type TemplateFuncContext struct {
	a   *Action
	svc *launchr.ServiceManager
}

// Action returns an [Action] related to the template.
func (ctx TemplateFuncContext) Action() *Action { return ctx.a }

// Services returns a [launchr.ServiceManager] for DI.
func (ctx TemplateFuncContext) Services() *launchr.ServiceManager { return ctx.svc }

// TemplateProcessors handles template processors used on an action load.
type TemplateProcessors struct {
	vproc  map[string]ValueProcessor
	tplFns map[string]any
}

// NewTemplateProcessors initializes TemplateProcessors with default functions.
func NewTemplateProcessors() *TemplateProcessors {
	p := &TemplateProcessors{
		vproc:  make(map[string]ValueProcessor),
		tplFns: make(map[string]any),
	}
	defaultTemplateFunc(p)
	return p
}

// defaultTemplateFunc defines template functions available during parsing of an action yaml.
func defaultTemplateFunc(p *TemplateProcessors) {
	// Returns a default value if v is nil or zero.
	p.AddTemplateFunc("default", func(v, d any) any {
		// Check IsEmpty method.
		if v, ok := v.(interface{ IsEmpty() bool }); ok && v.IsEmpty() {
			return d
		}

		// Check zero value, for example, empty string, 0, false,
		// or in the case of structs that all fields are zero values.
		if reflect.ValueOf(v).IsZero() {
			return d
		}

		// Checks if value is nil.
		if v == nil {
			return d
		}

		return v
	})

	// Checks if a value is nil. Used in conditions.
	p.AddTemplateFunc("isNil", func(v any) bool {
		return v == nil
	})

	// Checks if a value is not nil. Used in conditions.
	p.AddTemplateFunc("isSet", func(v any) bool {
		return v != nil
	})

	// Checks if a value is changed. Used in conditions.
	p.AddTemplateFunc("isChanged", func(ctx TemplateFuncContext) any {
		return func(v any) bool {
			name, ok := v.(string)
			if !ok {
				return false
			}
			input := ctx.Action().Input()
			return input.IsOptChanged(name) || input.IsArgChanged(name)
		}
	})

	// Mask a value in the output in case it's sensitive.
	p.AddTemplateFunc("mask", func(ctx TemplateFuncContext) any {
		var mask *launchr.SensitiveMask
		return func(v string) string {
			// Initialize a masking service per action.
			if mask == nil {
				ctx.Services().Get(&mask)
				mask = mask.Clone()
				input := ctx.Action().Input()
				// TODO: Review. This may not work as expected with Term and Log.
				io := input.Streams()
				input.SetStreams(launchr.NewBasicStreams(io.In(), io.Out(), io.Err(), launchr.WithSensitiveMask(mask)))
			}
			mask.AddString(v)
			return v
		}
	})
}

// ServiceInfo implements [launchr.Service] interface.
func (m *TemplateProcessors) ServiceInfo() launchr.ServiceInfo {
	return launchr.ServiceInfo{}
}

// ServiceCreate implements [launchr.ServiceCreate] interface.
func (m *TemplateProcessors) ServiceCreate(_ *launchr.ServiceManager) launchr.Service {
	return NewTemplateProcessors()
}

// AddValueProcessor adds processor to list of available processors
func (m *TemplateProcessors) AddValueProcessor(name string, vp ValueProcessor) {
	if _, ok := m.vproc[name]; ok {
		panic(fmt.Sprintf("value processor %q with the same name already exists", name))
	}
	m.vproc[name] = vp
}

// GetValueProcessors returns list of available processors
func (m *TemplateProcessors) GetValueProcessors() map[string]ValueProcessor {
	return m.vproc
}

// AddTemplateFunc registers a template function used on [inputProcessor].
func (m *TemplateProcessors) AddTemplateFunc(name string, fn any) {
	if _, ok := m.tplFns[name]; ok {
		panic(fmt.Sprintf("template function %q with the same name already exists", name))
	}
	m.tplFns[name] = fn
}

// GetTemplateFuncMap returns list of template functions used on [inputProcessor].
func (m *TemplateProcessors) GetTemplateFuncMap(ctx TemplateFuncContext) template.FuncMap {
	tplFuncMap := template.FuncMap{}
	for k, v := range m.tplFns {
		switch v := v.(type) {
		case func(ctx TemplateFuncContext) any:
			// Template function with action context.
			tplFuncMap[k] = v(ctx)

		default:
			// Usual template function processor.
			tplFuncMap[k] = v
		}
	}
	return tplFuncMap
}

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
	Input     *Input        // Input represents the associated action input in the current context.
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
