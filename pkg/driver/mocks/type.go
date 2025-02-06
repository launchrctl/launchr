// Package mocks holds the generated mocks for containers.
//
//go:generate go run go.uber.org/mock/mockgen@latest -destination=mock.go -package=mocks . ContainerRuntime
package mocks

import "github.com/launchrctl/launchr/pkg/driver"

// ContainerRuntime is a combined interface for the generated mock.
type ContainerRuntime interface {
	driver.ContainerRunner
	driver.ContainerRunnerSELinux
	driver.ContainerImageBuilder
}
