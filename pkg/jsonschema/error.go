package jsonschema

import (
	"fmt"
	"sort"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/launchrctl/launchr/internal/launchr"
)

// ErrSchemaValidationArray is an array of validation errors.
type ErrSchemaValidationArray []ErrSchemaValidation

// ErrSchemaValidation is a validation error.
type ErrSchemaValidation struct {
	// Path is a key path to the property.
	Path []string
	// Msg is an error message.
	Msg string

	// key is a sortable key.
	key string
}

// Error implements error interface.
func (err ErrSchemaValidationArray) Error() string {
	msgs := make([]string, len(err))
	for i := 0; i < len(err); i++ {
		msgs[i] = err[i].Error()
	}
	return fmt.Sprintf("validation errors:\n  - %s", strings.Join(msgs, "\n  - "))
}

// NewErrSchemaValidation creates a new error.
func NewErrSchemaValidation(path []string, msg string) ErrSchemaValidation {
	return ErrSchemaValidation{
		Path: path,
		Msg:  msg,
		key:  strings.Join(path, "/"),
	}
}

// Error implements error interface.
func (err ErrSchemaValidation) Error() string {
	return fmt.Sprintf("%s: %s", err.Path, err.Msg)
}

// newSchemaValidationErrors creates our error from jsonschema lib.
func newSchemaValidationErrors(err *jsonschema.ValidationError) ErrSchemaValidationArray {
	sl := collectNestedValidationErrors(err)
	// Sort errors by property name.
	sort.Slice(sl, func(i, j int) bool {
		return sl[i].key < sl[j].key
	})
	return sl
}

// collectNestedValidationErrors creates a plain slice of nested validation errors.
func collectNestedValidationErrors(err *jsonschema.ValidationError) []ErrSchemaValidation {
	if err.Causes == nil {
		return []ErrSchemaValidation{
			NewErrSchemaValidation(err.InstanceLocation, err.ErrorKind.LocalizedString(launchr.DefaultTextPrinter)),
		}
	}
	res := make([]ErrSchemaValidation, 0, len(err.Causes))
	for i := 0; i < len(err.Causes); i++ {
		res = append(res, collectNestedValidationErrors(err.Causes[i])...)
	}
	return res
}
