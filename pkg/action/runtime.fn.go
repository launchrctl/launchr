package action

import (
	"context"

	"github.com/launchrctl/launchr/internal/launchr"
)

// FnRuntimeCallback is a function type used in [FnRuntime].
type FnRuntimeCallback func(ctx context.Context, a *Action) error

// FnRuntime is a function type implementing [Runtime].
type FnRuntime struct {
	logger *launchr.Logger
	frc    func(ctx context.Context, a *Action) error
}

// NewFnRuntime creates runtime as a go function.
func NewFnRuntime(fn FnRuntimeCallback) Runtime {
	return &FnRuntime{frc: fn}
}

// Clone implements [Runtime] interface.
func (fn *FnRuntime) Clone() Runtime {
	return fn
}

// Init implements [Runtime] interface.
func (fn *FnRuntime) Init(_ context.Context, _ *Action) error {
	return nil
}

// SetLogger implements [Runtime] interface.
func (fn *FnRuntime) SetLogger(l *launchr.Logger) {
	fn.logger = l
}

// Log implements [Runtime] interface.
func (fn *FnRuntime) Log(attrs ...any) *launchr.Slog {
	return fn.logger.With(attrs...)
}

// Execute implements [Runtime] interface.
func (fn *FnRuntime) Execute(ctx context.Context, a *Action) error {
	launchr.Log().Debug("starting execution of the action", "run_env", "fn", "action_id", a.ID)
	return fn.frc(ctx, a)
}

// Close implements [Runtime] interface.
func (fn *FnRuntime) Close() error {
	return nil
}
