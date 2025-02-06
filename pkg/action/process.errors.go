package action

import (
	"fmt"

	"github.com/launchrctl/launchr/pkg/jsonschema"
)

// ErrValueProcessorNotApplicable error when the given parameter is not supported [ValueProcessor].
type ErrValueProcessorNotApplicable struct {
	Processor string
	Type      jsonschema.Type
}

func (err ErrValueProcessorNotApplicable) Error() string {
	return fmt.Sprintf("invalid value processor: %q can't be applied to a parameter of type %s", err.Processor, err.Type)
}

// ErrValueProcessorNotExist error when [ValueProcessor] is not registered in the app.
type ErrValueProcessorNotExist string

func (err ErrValueProcessorNotExist) Error() string {
	return fmt.Sprintf("requested value processor %q doesn't exist", string(err))
}

// ErrValueProcessorHandler error when [ValueProcessorHandler] processing has failed.
type ErrValueProcessorHandler struct {
	Processor string
	Param     string
	Err       error
}

func (err ErrValueProcessorHandler) Error() string {
	return fmt.Sprintf("error on processing parameter %q with %q: %s", err.Param, err.Processor, err.Err)
}

// Is implements [errors.Is] interface.
func (err ErrValueProcessorHandler) Is(cmp error) bool {
	return cmp.Error() == err.Err.Error() || cmp.Error() == err.Error()
}

// ErrValueProcessorOptionsValidation error when [ValueProcessorOptions] fails to validate.
type ErrValueProcessorOptionsValidation struct {
	Processor string
	Err       error
}

func (err ErrValueProcessorOptionsValidation) Error() string {
	return fmt.Sprintf("invalid %q value processor options: %s", err.Processor, err.Err)
}

// Is implements [errors.Is] interface.
func (err ErrValueProcessorOptionsValidation) Is(cmp error) bool {
	return cmp.Error() == err.Err.Error() || cmp.Error() == err.Error()
}

// ErrValueProcessorOptionsFieldValidation error when [ValueProcessorOptions] field validation error
type ErrValueProcessorOptionsFieldValidation struct {
	Field  string
	Reason string
}

func (err ErrValueProcessorOptionsFieldValidation) Error() string {
	switch err.Reason {
	case "":
		return fmt.Sprintf("option '%s' is invalid", err.Field)
	default:
		return fmt.Sprintf("option '%s' is %s", err.Field, err.Reason)
	}
}
