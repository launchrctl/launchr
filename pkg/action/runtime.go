package action

import (
	"context"

	"github.com/launchrctl/launchr/internal/launchr"
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
	// GetFlags returns flags group of runtime configuration.
	GetFlags() *FlagsGroup
	// SetFlags sets environment configuration.
	SetFlags(input *Input) error
	// ValidateInput validates input arguments in action definition.
	ValidateInput(input *Input) error
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
	Log() *launchr.Logger
	// LogWith returns runtime logger with attributes
	LogWith(attrs ...any) *launchr.Logger
}

// WithLogger provides a composition with log utilities.
type WithLogger struct {
	logger *launchr.Logger
	// logWith contains context arguments for a structured logger.
	logWith []any
}

// SetLogger implements [RuntimeLoggerAware] interface
func (c *WithLogger) SetLogger(l *launchr.Logger) {
	c.logger = l
}

// Log implements [RuntimeLoggerAware] interface
func (c *WithLogger) Log() *launchr.Logger {
	if c.logWith == nil {
		return launchr.Log()
	}
	return c.logger
}

// LogWith implements [RuntimeLoggerAware] interface
func (c *WithLogger) LogWith(attrs ...any) *launchr.Logger {
	if attrs != nil {
		c.logWith = append(c.logWith, attrs...)
	}

	return &launchr.Logger{
		Slog:       c.Log().With(c.logWith...),
		LogOptions: c.Log().LogOptions,
	}
}

// RuntimeTermAware is an interface for term runtime.
type RuntimeTermAware interface {
	Runtime
	// SetTerm adds runtime terminal
	SetTerm(t *launchr.Terminal)
	// Term returns runtime terminal.
	Term() *launchr.Terminal
}

// WithTerm provides a composition with term utilities.
type WithTerm struct {
	term *launchr.Terminal
}

// SetTerm implements [RuntimeTermAware] interface
func (c *WithTerm) SetTerm(t *launchr.Terminal) {
	c.term = t
}

// Term implements [RuntimeTermAware] interface
func (c *WithTerm) Term() *launchr.Terminal {
	if c.term == nil {
		return launchr.Term()
	}
	return c.term
}

// WithFlagsGroup provides a composition with flags utilities.
type WithFlagsGroup struct {
	flags *FlagsGroup
}

// SetFlagsGroup sets flags group to work with
func (c *WithFlagsGroup) SetFlagsGroup(group *FlagsGroup) {
	c.flags = group
}

// GetFlagsGroup returns flags group
func (c *WithFlagsGroup) GetFlagsGroup() *FlagsGroup {
	return c.flags
}
