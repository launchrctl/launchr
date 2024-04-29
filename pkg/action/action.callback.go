package action

import (
	"context"
	"github.com/launchrctl/launchr/pkg/cli"
)

type CallbackAction struct {
	baseAction

	callback FnAction
	SomeVar  string
}

// FnAction ...
type FnAction func(input map[string]interface{}, services map[string]interface{}) error

// SetInput saves arguments and options for later processing in run, templates, etc.
func (a *CallbackAction) SetInput(_ Input) (err error) {
	//panic("todo")
	return nil
}

// Execute runs action in the specified environment.
func (a *CallbackAction) Execute(ctx context.Context) error {
	var act Action = a
	return a.baseAction.execute(ctx, act)
}

func (a *CallbackAction) EnsureLoaded() (err error) {
	return nil
}

// Clone returns a copy of an action.
func (a *CallbackAction) Clone() Action {
	if a == nil {
		return nil
	}
	c := &CallbackAction{
		baseAction: baseAction{
			ID:  a.ID,
			def: a.def,
		},
		callback: a.GetCallback(),
		SomeVar:  a.SomeVar,
	}
	return c
}

func (a *CallbackAction) GetCallback() FnAction {
	return a.callback
}

// NewCallbackAction creates a new action.
func NewCallbackAction(id string, definition *Definition) *CallbackAction {
	var exampleFunc FnAction = func(input map[string]interface{}, services map[string]interface{}) error {
		cli.Println("Executing example function...")

		return nil // return nil error for successful execution
	}

	return &CallbackAction{
		baseAction: baseAction{
			ID:  id,
			def: definition,
		},
		callback: exampleFunc,
		SomeVar:  "abc",
	}
}
