package action

import (
	"github.com/launchrctl/launchr/pkg/jsonschema"
)

// ValueProcessor defines an interface for processing a value based on its type and some options.
type ValueProcessor interface {
	IsApplicable(valueType jsonschema.Type) bool
	Execute(value any, options map[string]any) (any, error)
}

// ValueProcessorFn is a function signature used as a callback in processors.
type ValueProcessorFn func(value any, options map[string]any) (any, error)

// NewFuncProcessor creates a new instance of [FuncProcessor] with the specified formats and callback.
func NewFuncProcessor(formats []jsonschema.Type, callback ValueProcessorFn) FuncProcessor {
	return FuncProcessor{
		applicableFormats: formats,
		callback:          callback,
	}
}

// FuncProcessor represents a processor that applies a callback function to values based on certain applicable formats.
type FuncProcessor struct {
	applicableFormats []jsonschema.Type
	callback          ValueProcessorFn
}

// IsApplicable checks if the given valueType is present in the applicableFormats slice of the [FuncProcessor].
func (p FuncProcessor) IsApplicable(valueType jsonschema.Type) bool {
	for _, item := range p.applicableFormats {
		if valueType == item {
			return true
		}
	}

	return false
}

// Execute applies the callback function of the [FuncProcessor] to the given value with options.
func (p FuncProcessor) Execute(value any, options map[string]any) (any, error) {
	return p.callback(value, options)
}
