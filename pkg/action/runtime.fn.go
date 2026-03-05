package action

import (
	"context"
)

// FnRuntimeCallback is a function type used in [FnRuntime].
type FnRuntimeCallback func(ctx context.Context, a *Action) error

// FnRuntimeCallbackWithResult is a function type that returns a structured result.
type FnRuntimeCallbackWithResult func(ctx context.Context, a *Action) (any, error)

// FnRuntime is a function type implementing [Runtime].
type FnRuntime struct {
	WithLogger
	WithTerm
	WithResult

	fn       FnRuntimeCallback
	fnResult FnRuntimeCallbackWithResult
}

// NewFnRuntime creates runtime as a go function.
func NewFnRuntime(fn FnRuntimeCallback) Runtime {
	return &FnRuntime{fn: fn}
}

// NewFnRuntimeWithResult creates runtime as a go function that returns a structured result.
// The result can be retrieved via the RuntimeResultProvider interface after execution.
func NewFnRuntimeWithResult(fn FnRuntimeCallbackWithResult) Runtime {
	return &FnRuntime{fnResult: fn}
}

// Clone implements [Runtime] interface.
func (fn *FnRuntime) Clone() Runtime {
	if fn.fnResult != nil {
		return NewFnRuntimeWithResult(fn.fnResult)
	}
	return NewFnRuntime(fn.fn)
}

// Init implements [Runtime] interface.
func (fn *FnRuntime) Init(_ context.Context, _ *Action) error {
	return nil
}

// Execute implements [Runtime] interface.
func (fn *FnRuntime) Execute(ctx context.Context, a *Action) error {
	fn.Log().Debug("starting execution of the action", "run_env", "fn", "action_id", a.ID)
	if fn.fnResult != nil {
		result, err := fn.fnResult(ctx, a)
		fn.SetResult(result)
		return err
	}
	return fn.fn(ctx, a)
}

// Close implements [Runtime] interface.
func (fn *FnRuntime) Close() error {
	return nil
}
