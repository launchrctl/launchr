package action

import (
	"context"
	"path/filepath"

	"github.com/launchrctl/launchr/pkg/cli"
	"github.com/launchrctl/launchr/pkg/types"
)

// Action is an action definition with a contextual id (name), working directory path
// and a runtime context such as input arguments and options.
type Action struct {
	ID     string
	Loader Loader
	wd     string
	fpath  string
	def    *Definition

	env   RunEnvironment
	input Input
}

// Input is a container for action input arguments and options.
type Input struct {
	Args TypeArgs
	Opts TypeOpts
	IO   cli.Streams // @todo should it be in Input?
}

type (
	// TypeArgs is a type alias for action arguments.
	TypeArgs = map[string]string
	// TypeOpts is a type alias for action options.
	TypeOpts = map[string]interface{}
)

// NewAction creates a new action.
func NewAction(id, wd, fpath string) *Action {
	return &Action{
		ID:    id,
		wd:    wd,
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
		fpath:  a.fpath,
		Loader: a.Loader,
	}
	return c
}

// Reset unsets loaded action to force reload.
func (a *Action) Reset() { a.def = nil }

// GetInput returns action input.
func (a *Action) GetInput() Input { return a.input }

// Filepath returns action file path.
func (a *Action) Filepath() string { return a.fpath }

// Dir returns an action file directory.
func (a *Action) Dir() string { return filepath.Dir(filepath.Join(a.wd, a.Filepath())) }

// SetRunEnvironment sets environment to run the action.
func (a *Action) SetRunEnvironment(env RunEnvironment) { a.env = env }

// DefinitionEncoded returns encoded action file content.
func (a *Action) DefinitionEncoded() ([]byte, error) { return a.Loader.Content() }

// EnsureLoaded loads an action file with replaced arguments and options.
func (a *Action) EnsureLoaded() (err error) {
	if a.def != nil {
		return err
	}
	a.def, err = a.Loader.Load()
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
func (a *Action) SetInput(input Input) error {
	if err := a.ValidateInput(input); err != nil {
		return err
	}
	a.input = input
	// Reset to load the action file again with new replacements.
	a.Reset()
	return a.EnsureLoaded()
}

// ValidateInput validates arguments and options according to
// a specified json schema in action definition.
func (a *Action) ValidateInput(_ Input) error {
	// @todo implement json schema validation
	//js := a.JSONSchema()
	// @todo validate must have info about which fields failed.
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
