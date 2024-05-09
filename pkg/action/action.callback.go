package action

import (
	"context"
)

type CallbackAction struct {
	baseAction

	callback ServiceCallbackFunc
}

// ServiceCallbackFunc ...
type ServiceCallbackFunc func(input Input) error

// SetInput saves arguments and options for later processing in run, templates, etc.
func (a *CallbackAction) SetInput(input Input) (err error) {
	err = a.processArgs(input.Args)
	if err != nil {
		return err
	}

	err = a.processOptions(input.Opts)
	if err != nil {
		return err
	}

	a.input = input

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
	}
	return c
}

func (a *CallbackAction) GetCallback() ServiceCallbackFunc {
	return a.callback
}

// NewCallbackAction creates a new action.
func NewCallbackAction(id string, definition *Definition, callback ServiceCallbackFunc) *CallbackAction {
	return &CallbackAction{
		baseAction: baseAction{
			ID:  id,
			def: definition,
		},
		callback: callback,
	}
}
