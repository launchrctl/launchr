package action

import (
	"context"

	"github.com/launchrctl/launchr/internal/launchr"
)

// FnRuntime is a function type implementing [Runtime].
type FnRuntime func(ctx context.Context, a *Action) error

// NewFnRuntime creates runtime as a go function.
func NewFnRuntime(fn FnRuntime) Runtime {
	return fn
}

// Clone implements [Runtime] interface.
func (fn FnRuntime) Clone() Runtime {
	return fn
}

// Init implements [Runtime] interface.
func (fn FnRuntime) Init(_ context.Context, _ *Action) error {
	return nil
}

// Execute implements [Runtime] interface.
func (fn FnRuntime) Execute(ctx context.Context, a *Action) error {
	launchr.Log().Debug("starting execution of the action", "run_env", "fn", "action_id", a.ID)
	return fn(ctx, a)
}

// Close implements [Runtime] interface.
func (fn FnRuntime) Close() error {
	return nil
}
