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
		panic("test")
	}

	cli.Println("%v", act)
	cli.Println("%s", act.SomeVar)
	cli.Println("%s", act.GetID())
	cli.Println("%s", (*a).ActionDef().Title)

	cli.Println("works")
	map1 := make(map[string]interface{})
	map2 := make(map[string]interface{})

	call := act.GetCallback()
	err = call(map1, map2)

	return err
}

func (c *FunctionEnv) Close() error {
	return nil
}
