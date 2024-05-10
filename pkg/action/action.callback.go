package action

import (
	"context"
	"fmt"
)

// CallbackActionType is an action type name for CallbackAction.
const CallbackActionType = "callback"

// CallbackAction is an action definition with purpose to work as function.
type CallbackAction struct {
	baseAction

	callback ServiceCallbackFunc
}

// ServiceCallbackFunc ...
type ServiceCallbackFunc func(ctx context.Context, input Input) error

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
	return a.baseAction.execute(ctx, a)
}

// EnsureLoaded loads an action file with replaced arguments and options.
func (a *CallbackAction) EnsureLoaded() (err error) {
	if a.def == nil {
		return fmt.Errorf("invalid action definition provided for %s", a.GetID())
	}

	if a.def.Action.Target != CallbackActionType {
		return fmt.Errorf("invalid action definition for %s, wrong `target`, expected %s", a.GetID(), CallbackActionType)
	}

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

// GetCallback returns actions' ServiceCallbackFunc.
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
