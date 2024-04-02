package action

import (
	"github.com/launchrctl/launchr/internal/launchr"
	"github.com/launchrctl/launchr/pkg/jsonschema"
)

// NewFuncProcessor creates a new instance of FuncProcessor with the specified formats and callback.
func NewFuncProcessor(formats []jsonschema.Type, callback launchr.ValueProcessorFn) FuncProcessor {
	return FuncProcessor{
		applicableFormats: formats,
		callback:          callback,
	}
}

// FuncProcessor represents a processor that applies a callback function to values based on certain applicable formats.
type FuncProcessor struct {
	applicableFormats []jsonschema.Type
	callback          launchr.ValueProcessorFn
}

// IsApplicable checks if the given valueType is present in the applicableFormats slice of the FuncProcessor.
func (p FuncProcessor) IsApplicable(valueType jsonschema.Type) bool {
	for _, item := range p.applicableFormats {
		if valueType == item {
			return true
		}
	}

	return false
}

// Execute applies the callback function of the FuncProcessor to the given value with options.
func (p FuncProcessor) Execute(value interface{}, options map[string]interface{}) (interface{}, error) {
	return p.callback(value, options)
}
