package action

import (
	"context"
	"github.com/launchrctl/launchr/pkg/cli"
)

type FunctionEnv struct {
}

// NewFunctionEnvironment creates a new action Docker environment.
func NewFunctionEnvironment() RunEnvironment {
	return &FunctionEnv{}
}

func (c *FunctionEnv) Init() error {
	return nil
}

func (c *FunctionEnv) Execute(ctx context.Context, a *Action) (err error) {
	ctx, cancelFn := context.WithCancel(ctx)
	defer cancelFn()
	if err = c.Init(); err != nil {
		return err
	}

	act, ok := (*a).(*CallbackAction)
	if !ok {
		panic("not supported action type submitted to env")
	}

	cli.Println("%v", act)
	cli.Println("%s", act.GetID())
	cli.Println("%s", act.ActionDef().Title)

	cli.Println("works")

	call := act.GetCallback()
	err = call(act.GetInput())

	return err
}

func (c *FunctionEnv) Close() error {
	return nil
}
