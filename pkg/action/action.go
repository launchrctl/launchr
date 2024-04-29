package action

import (
	"context"
	"errors"
	"fmt"

	"github.com/launchrctl/launchr/pkg/cli"
	"github.com/launchrctl/launchr/pkg/jsonschema"
)

var (
	errInvalidProcessor          = errors.New("invalid configuration, processor is required")
	errTplNotApplicableProcessor = "invalid configuration, processor can't be applied to value of type %s"
	errTplNonExistProcessor      = "requested processor %q doesn't exist"
)

// Action is an action definition with a contextual id (name), working directory path
// and a runtime context such as input arguments and options.
type Action interface {
	GetID() string
	ActionDef() *DefAction
	SetRunEnvironment(env RunEnvironment)
	GetRunEnvironment() RunEnvironment
	GetInput() Input
	SetInput(input Input) (err error)
	EnsureLoaded() (err error)
	Execute(ctx context.Context) error
	SetProcessors(list map[string]ValueProcessor)
	GetProcessors() map[string]ValueProcessor
	JSONSchema() jsonschema.Schema
	Clone() Action
}

type baseAction struct {
	ID  string      // ID is an action unique id compiled from path.
	def *Definition // def is an action definition.

	env        RunEnvironment            // env is the run environment driver to execute the action.
	input      Input                     // input is a container for env variables.
	processors map[string]ValueProcessor // processors are ValueProcessors for manipulating input.
}

// GetID returns action unique ID
func (a *baseAction) GetID() string {
	return a.ID
}

func (a *baseAction) execute(ctx context.Context, act Action) error {
	// @todo maybe it shouldn't be here.
	if a.env == nil {
		panic("run environment is not set, call SetRunEnvironment first")
	}
	defer a.env.Close()

	return a.env.Execute(ctx, act)
}

// GetInput returns action input.
func (a *baseAction) GetInput() Input { return a.input }

// SetRunEnvironment sets environment to run the action.
func (a *baseAction) SetRunEnvironment(env RunEnvironment) { a.env = env }

// GetRunEnvironment returns action run environment.
func (a *baseAction) GetRunEnvironment() RunEnvironment { return a.env }

// ActionDef returns action definition with replaced variables.
func (a *baseAction) ActionDef() *DefAction {
	if a.def == nil {
		panic("action data is not available")
	}
	return a.def.Action
}

// SetProcessors sets the value processors for an Action.
func (a *baseAction) SetProcessors(list map[string]ValueProcessor) {
	a.processors = list
}

// GetProcessors returns processors map.
func (a *baseAction) GetProcessors() map[string]ValueProcessor {
	return a.processors
}

func (a *baseAction) processOptions(opts TypeOpts) error {
	for _, optDef := range a.ActionDef().Options {
		if _, ok := opts[optDef.Name]; !ok {
			continue
		}

		value := opts[optDef.Name]
		toApply := optDef.Process

		value, err := a.processValue(value, optDef.Type, toApply)
		if err != nil {
			return err
		}

		opts[optDef.Name] = value
	}

	return nil
}

func (a *baseAction) processArgs(args TypeArgs) error {
	for _, argDef := range a.ActionDef().Arguments {
		if _, ok := args[argDef.Name]; !ok {
			continue
		}

		value := args[argDef.Name]
		toApply := argDef.Process
		value, err := a.processValue(value, argDef.Type, toApply)
		if err != nil {
			return err
		}

		args[argDef.Name] = value
	}

	return nil
}

func (a *baseAction) processValue(value interface{}, valueType jsonschema.Type, toApplyProcessors []ValueProcessDef) (interface{}, error) {
	newValue := value
	processors := a.GetProcessors()

	for _, processor := range toApplyProcessors {
		if processor.Processor == "" {
			return value, errInvalidProcessor
		}

		proc, ok := processors[processor.Processor]
		if !ok {
			return value, fmt.Errorf(errTplNonExistProcessor, processor.Processor)
		}

		if !proc.IsApplicable(valueType) {
			return value, fmt.Errorf(errTplNotApplicableProcessor, valueType)
		}

		processedValue, err := proc.Execute(newValue, processor.Options)
		if err != nil {
			return value, err
		}

		newValue = processedValue
	}

	return newValue, nil
}

// Input is a container for action input arguments and options.
type Input struct {
	Args    TypeArgs
	Opts    TypeOpts
	IO      cli.Streams // @todo should it be in Input?
	ArgsRaw []string
}

// ValidateInput validates input arguments in action definition.
func (a *baseAction) ValidateInput(args TypeArgs) error {
	argsInitNum := len(a.ActionDef().Arguments)
	argsInputNum := len(args)
	if argsInitNum != argsInputNum {
		return fmt.Errorf("accepts %d arg(s), received %d", argsInitNum, argsInputNum)
	}

	return nil
}

type (
	// TypeArgs is a type alias for action arguments.
	TypeArgs = map[string]interface{}
	// TypeOpts is a type alias for action options.
	TypeOpts = map[string]interface{}
)
