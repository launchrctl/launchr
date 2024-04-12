package action

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"

	jsvalidate "github.com/santhosh-tekuri/jsonschema/v5"

	"github.com/launchrctl/launchr/pkg/cli"
	"github.com/launchrctl/launchr/pkg/jsonschema"
	"github.com/launchrctl/launchr/pkg/types"
)

var (
	errInvalidProcessor          = errors.New("invalid configuration, processor is required")
	errTplNotApplicableProcessor = "invalid configuration, processor can't be applied to value of type %s"
	errTplNonExistProcessor      = "requested processor %q doesn't exist"
)

// Action is an action definition with a contextual id (name), working directory path
// and a runtime context such as input arguments and options.
type Action struct {
	ID     string // ID is an action unique id compiled from path.
	Loader Loader // Loader is a function to load action definition. Helpful to reload with replaced variables.

	// wd is a working directory set from app level.
	// Usually current working directory, but may be overridden by a plugin.
	wd    string
	fsdir string      // fsdir is a base directory where the action was discovered (for better ID naming).
	fpath string      // fpath is a path to action definition file.
	def   *Definition // def is an action definition. Loaded by Loader, may be nil when not initialized.

	env        RunEnvironment            // env is the run environment driver to execute the action.
	input      Input                     // input is a container for env variables.
	processors map[string]ValueProcessor // processors are ValueProcessors for manipulating input.
}

// Input is a container for action input arguments and options.
type Input struct {
	Args    TypeArgs
	Opts    TypeOpts
	IO      cli.Streams // @todo should it be in Input?
	ArgsRaw []string
}

type (
	// TypeArgs is a type alias for action arguments.
	TypeArgs = map[string]interface{}
	// TypeOpts is a type alias for action options.
	TypeOpts = map[string]interface{}
)

// NewAction creates a new action.
func NewAction(id, wd, fsdir, fpath string) *Action {
	return &Action{
		ID:    id,
		wd:    wd,
		fsdir: fsdir,
		fpath: fpath,
	}
}

// Clone returns a copy of an action.
func (a *Action) Clone() *Action {
	if a == nil {
		return nil
	}
	c := &Action{
		ID:     a.ID,
		wd:     a.wd,
		fsdir:  a.fsdir,
		fpath:  a.fpath,
		Loader: a.Loader,
	}
	return c
}

// SetProcessors sets the value processors for an Action.
func (a *Action) SetProcessors(list map[string]ValueProcessor) {
	a.processors = list
}

// GetProcessors returns processors map.
func (a *Action) GetProcessors() map[string]ValueProcessor {
	return a.processors
}

// Reset unsets loaded action to force reload.
func (a *Action) Reset() { a.def = nil }

// GetInput returns action input.
func (a *Action) GetInput() Input { return a.input }

// WorkDir returns action working directory.
func (a *Action) WorkDir() string {
	if a.def != nil && a.def.WD != "" {
		wd, err := filepath.Abs(filepath.Clean(a.def.WD))
		if err == nil {
			return wd
		}
	}
	return a.wd
}

// Filepath returns action file path.
func (a *Action) Filepath() string { return a.fpath }

// Dir returns an action file directory.
func (a *Action) Dir() string { return filepath.Dir(a.Filepath()) }

// SetRunEnvironment sets environment to run the action.
func (a *Action) SetRunEnvironment(env RunEnvironment) { a.env = env }

// DefinitionEncoded returns encoded action file content.
func (a *Action) DefinitionEncoded() ([]byte, error) { return a.Loader.Content() }

// EnsureLoaded loads an action file with replaced arguments and options.
func (a *Action) EnsureLoaded() (err error) {
	if a.def != nil {
		return err
	}
	a.def, err = a.Loader.Load(LoadContext{Action: a})
	return err
}

// ActionDef returns action definition with replaced variables.
func (a *Action) ActionDef() *DefAction {
	if a.def == nil {
		panic("action data is not available, call \"EnsureLoaded\" method first to load the data")
	}
	return a.def.Action
}

// ImageBuildInfo implements ImageBuildResolver.
func (a *Action) ImageBuildInfo(image string) *types.BuildDefinition {
	return a.ActionDef().Build.ImageBuildInfo(image, a.Dir())
}

// SetInput saves arguments and options for later processing in run, templates, etc.
func (a *Action) SetInput(input Input) (err error) {
	if err = a.EnsureLoaded(); err != nil {
		return err
	}
	// @todo disabled for now until fully tested.
	//if err = a.validateJSONSchema(input); err != nil {
	//	return err
	//}

	err = a.processArgs(input.Args)
	if err != nil {
		return err
	}

	err = a.processOptions(input.Opts)
	if err != nil {
		return err
	}

	a.input = input
	// Reset to load the action file again with new replacements.
	a.Reset()
	return a.EnsureLoaded()
}

// validateJSONSchema validates arguments and options according to
func (a *Action) processOptions(opts TypeOpts) error {
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

func (a *Action) processArgs(args TypeArgs) error {
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

func (a *Action) processValue(value interface{}, valueType jsonschema.Type, toApplyProcessors []ValueProcessDef) (interface{}, error) {
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

// validateJSONSchema validates arguments and options according to
// a specified json schema in action definition.
// @todo move to jsonschema
func (a *Action) validateJSONSchema(inp Input) error {
	jsch := a.JSONSchema()
	// @todo cache jsonschema and resources.
	b, err := json.Marshal(jsch)
	if err != nil {
		return err
	}
	buf := bytes.NewBuffer(b)
	c := jsvalidate.NewCompiler()
	err = c.AddResource(a.Filepath(), buf)
	if err != nil {
		return err
	}
	sch, err := c.Compile(a.Filepath())
	if err != nil {
		return err
	}
	err = sch.Validate(map[string]interface{}{
		"arguments": inp.Args,
		"options":   inp.Opts,
	})
	if err != nil {
		return err
	}
	// @todo validate must have info about which fields failed.
	return nil
}

// ValidateInput validates input arguments in action definition.
func (a *Action) ValidateInput(args TypeArgs) error {
	argsInitNum := len(a.ActionDef().Arguments)
	argsInputNum := len(args)
	if argsInitNum != argsInputNum {
		return fmt.Errorf("expected (%d) arg(s), provided (%d) arg(s)", argsInitNum, argsInputNum)
	}

	return nil
}

// Execute runs action in the specified environment.
func (a *Action) Execute(ctx context.Context) error {
	// @todo maybe it shouldn't be here.
	if a.env == nil {
		panic("run environment is not set, call SetRunEnvironment first")
	}
	defer a.env.Close()
	return a.env.Execute(ctx, a)
}
