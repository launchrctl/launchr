package driver

import (
	"fmt"
)

// Type defines implemented driver types.
type Type string

const (
	Docker Type = "docker" // Docker driver
)

// New creates a new driver based on a type.
func New(t Type) (ContainerRunner, error) {
	switch t {
	case Docker:
		return NewDockerDriver()
	default:
		panic(fmt.Sprintf("driver \"%s\" is not implemented", t))
	}
}
