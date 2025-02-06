package launchr

import (
	"reflect"
	"unsafe"
)

// privateFieldValue accesses private struct field of external package using reflect and unsafe.
func privateFieldValue[T any](v any, fieldName string) T {
	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	// Access the field by name
	field := val.FieldByName(fieldName)
	if !field.IsValid() {
		panic("Invalid field: " + fieldName)
	}

	// Access the field's memory directly using unsafe.Pointer
	//nolint:gosec // G103: We need it to get the value.
	ptr := unsafe.Pointer(field.UnsafeAddr())
	return reflect.NewAt(field.Type(), ptr).Elem().Interface().(T)
}

// typeString returns a type string with full package name.
func typeString(v any) string {
	return reflect.TypeOf(v).String()
}
