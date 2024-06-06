package action

import (
	"context"
)

type functionEnv struct {
}

// NewFunctionEnvironment creates a new action Docker environment.
func NewFunctionEnvironment() RunEnvironment {
	return &functionEnv{}
}

// Init prepares the run environment.
func (c *functionEnv) Init() error {
	return nil
}

// Execute runs an action in the environment and operates with IO through streams.
func (c *functionEnv) Execute(ctx context.Context, a Action) (err error) {
	ctx, cancelFn := context.WithCancel(ctx)
	defer cancelFn()
	if err = c.Init(); err != nil {
		return err
	}

	act, ok := a.(*CallbackAction)
	if !ok {
		panic("not supported action type submitted to function env")
	}

	call := act.GetCallback()
	err = call(ctx, act.GetInput())

	return err
}

// Close does wrap up operations.
func (c *functionEnv) Close() error {
	return nil
}

func (c *functionEnv) ValidateInput(a Action, args TypeArgs) error {
	act, ok := a.(*CallbackAction)
	if !ok {
		panic("not supported action type submitted to container env")
	}

	// Check arguments if no exec flag present.
	return act.ValidateInput(args)
}
