package action

import (
	"context"

	"github.com/launchrctl/launchr/internal/launchr"
	"github.com/launchrctl/launchr/pkg/jsonschema"
)

// Runtime is an interface for action execution environment.
type Runtime interface {
	// Init prepares the runtime.
	Init(ctx context.Context, a *Action) error
	// Execute runs action `a` in the environment and operates with io through streams.
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
	// JSONSchema returns json schema of runtime flags.
	JSONSchema() jsonschema.Schema
	// ValidateJSONSchema validates options according to a specified json schema of [RuntimeFlags]
	ValidateJSONSchema(params InputParams) error
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

// RuntimeLoggerAware is an interface for logger runtime.
type RuntimeLoggerAware interface {
	Runtime
	// SetLogger adds runtime logger
	SetLogger(l *launchr.Logger)
	// Log returns runtime logger
	Log(attrs ...any) *launchr.Slog
	// SetTerm adds runtime terminal
	SetTerm(t *launchr.Terminal)
	// Term returns runtime terminal.
	Term() *launchr.Terminal
}

// RuntimeWithLogger provides a runtime composition with logging and terminal utilities.
type RuntimeWithLogger struct {
	term   *launchr.Terminal
	logger *launchr.Logger
	// logWith contains context arguments for a structured logger.
	logWith []any
}

// SetLogger implements [RuntimeLoggerAware] interface
func (c *RuntimeWithLogger) SetLogger(l *launchr.Logger) {
	c.logger = l
}

// Log implements [RuntimeLoggerAware] interface
func (c *RuntimeWithLogger) Log(attrs ...any) *launchr.Slog {
	if attrs != nil {
		c.logWith = append(c.logWith, attrs...)
	}
	return c.logger.With(c.logWith...)
}

// SetTerm implements [RuntimeLoggerAware] interface
func (c *RuntimeWithLogger) SetTerm(t *launchr.Terminal) {
	c.term = t
}

// Term implements [RuntimeLoggerAware] interface
func (c *RuntimeWithLogger) Term() *launchr.Terminal {
	return c.term
}
