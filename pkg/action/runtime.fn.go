package action

import (
	"context"
)

// FnRuntimeCallback is a function type used in [FnRuntime].
type FnRuntimeCallback func(ctx context.Context, a *Action) error

// FnRuntime is a function type implementing [Runtime].
type FnRuntime struct {
	WithLogger
	WithTerm

	fn FnRuntimeCallback
}

// NewFnRuntime creates runtime as a go function.
func NewFnRuntime(fn FnRuntimeCallback) Runtime {
	return &FnRuntime{fn: fn}
}

// Clone implements [Runtime] interface.
func (fn *FnRuntime) Clone() Runtime {
	return NewFnRuntime(fn.fn)
}

// Init implements [Runtime] interface.
func (fn *FnRuntime) Init(_ context.Context, _ *Action) error {
	return nil
}

// Execute implements [Runtime] interface.
func (fn *FnRuntime) Execute(ctx context.Context, a *Action) error {
	fn.Log().Debug("starting execution of the action", "run_env", "fn", "action_id", a.ID)
	return fn.fn(ctx, a)
}

// Close implements [Runtime] interface.
func (fn *FnRuntime) Close() error {
	return nil
}
