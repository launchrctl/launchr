package driver

import (
	"fmt"
)

// Type defines implemented driver types.
type Type string

// Available container runtime types.
const (
	Docker     Type = "docker"     // Docker runtime.
	Kubernetes Type = "kubernetes" // Kubernetes runtime.
)

// New creates a new driver based on a type.
func New(t Type) (ContainerRunner, error) {
	switch t {
	case Docker:
		return NewDockerDriver()
	default:
		panic(fmt.Sprintf("container runtime %q is not implemented", t))
	}
}
