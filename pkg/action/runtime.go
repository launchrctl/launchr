package action

import (
	"context"
)

// Runtime is an interface for action execution environment.
type Runtime interface {
	// Init prepares the runtime.
	Init(ctx context.Context, a *Action) error
	// Execute runs action a in the environment and operates with io through streams.
	Execute(ctx context.Context, a *Action) error
	// Close does wrap up operations.
	Close() error
	// Clone creates the same runtime, but in initial state.
	Clone() Runtime
}

// RuntimeFlags is an interface to define environment specific runtime configuration.
type RuntimeFlags interface {
	Runtime
	// FlagsDefinition provides definitions for action environment specific flags.
	FlagsDefinition() ParametersList
	// UseFlags sets environment configuration.
	UseFlags(flags InputParams) error
	// ValidateInput validates input arguments in action definition.
	ValidateInput(a *Action, input *Input) error
}

// ContainerRuntime is an interface for container runtime.
type ContainerRuntime interface {
	Runtime
	// SetContainerNameProvider sets container name provider.
	SetContainerNameProvider(ContainerNameProvider)
	// AddImageBuildResolver adds an image build resolver to a chain.
	AddImageBuildResolver(ImageBuildResolver)
	// SetImageBuildCacheResolver sets an image build cache resolver
	// to check when image must be rebuilt.
	SetImageBuildCacheResolver(*ImageBuildCacheResolver)
}
