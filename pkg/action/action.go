package action

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/launchrctl/launchr/pkg/jsonschema"
	"github.com/launchrctl/launchr/pkg/types"
)

var (
	errTplNotApplicableProcessor = "invalid configuration, processor can't be applied to value of type %s"
	errTplNonExistProcessor      = "requested processor %q doesn't exist"
)

// Action is an action definition with a contextual id (name), working directory path
// and a runtime context such as input arguments and options.
type Action struct {
	ID string // ID is a unique action id provided by [IDProvider].

	// loader is a function to load action definition.
	// Helpful to reload with replaced variables.
	loader Loader
	// wd is a working directory set from app level.
	// Usually current working directory, but may be overridden by a plugin.
	wd     string
	fsdir  string      // fsdir is a base directory where the action was discovered (for better ID idp).
	fpath  string      // fpath is a path to action definition file.
	def    *Definition // def is an action definition. Loaded by [Loader], may be nil when not initialized.
	defRaw *Definition // defRaw is a raw action definition. Loaded by [Loader], may be nil when not initialized.

	runtime    Runtime                   // runtime is the [Runtime] to execute the action.
	input      *Input                    // input is a storage for arguments and options used in runtime.
	processors map[string]ValueProcessor // processors are [ValueProcessor] for manipulating input.
}

// New creates a new action.
func New(idp IDProvider, l Loader, fsdir string, fpath string) *Action {
	// We don't define ID here because we use [Action] object for
	// context creation to calculate ID later.
	a := &Action{
		loader: l,
		fsdir:  fsdir,
		fpath:  fpath,
	}
	// Assign ID to an action.
	a.ID = idp.GetID(a)
	if a.ID == "" {
		panic(fmt.Errorf("action id cannot be empty, file %q", fpath))
	}
	a.SetWorkDir(".")
	return a
}

// NewFromYAML creates a new action from yaml content.
func NewFromYAML(id string, b []byte) *Action {
	return New(StringID(id), &YamlLoader{Bytes: b}, "", "")
}

// Clone returns a copy of an action.
func (a *Action) Clone() *Action {
	if a == nil {
		return nil
	}
	c := &Action{
		ID: a.ID,

		loader: a.loader,
		wd:     a.wd,
		fsdir:  a.fsdir,
		fpath:  a.fpath,
	}
	if a.runtime != nil {
		c.runtime = a.runtime.Clone()
	}
	return c
}

// SetProcessors sets the value processors for an [Action].
func (a *Action) SetProcessors(list map[string]ValueProcessor) {
	a.processors = list
}

// GetProcessors returns processors map.
func (a *Action) GetProcessors() map[string]ValueProcessor {
	return a.processors
}

// Reset unsets loaded action to force reload.
func (a *Action) Reset() { a.def = nil }

// Input returns action input.
func (a *Action) Input() *Input {
	if a.input == nil {
		// Return empty input for consistency to prevent nil call.
		return &Input{action: a}
	}
	return a.input
}

// SetWorkDir sets action working directory.
func (a *Action) SetWorkDir(wd string) {
	a.wd, _ = filepath.Abs(filepath.Clean(wd))
}

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
func (a *Action) Filepath() string { return filepath.Join(a.fsdir, a.fpath) }

// Dir returns an action file directory.
func (a *Action) Dir() string { return filepath.Dir(a.Filepath()) }

// Runtime returns environment to run the action.
func (a *Action) Runtime() Runtime { return a.runtime }

// SetRuntime sets environment to run the action.
func (a *Action) SetRuntime(r Runtime) { a.runtime = r }

// DefinitionEncoded returns encoded action file content.
func (a *Action) DefinitionEncoded() ([]byte, error) { return a.loader.Content() }

// Raw returns unprocessed action definition. It is faster and may produce fewer errors.
// It may be helpful if needed to peek inside the action file to read header.
func (a *Action) Raw() (*Definition, error) {
	var err error
	if a.defRaw == nil {
		a.defRaw, err = a.loader.LoadRaw()
	}
	return a.defRaw, err
}

// EnsureLoaded loads an action file with replaced arguments and options.
func (a *Action) EnsureLoaded() (err error) {
	if a.def != nil {
		return err
	}
	a.def, err = a.loader.Load(LoadContext{Action: a})
	return err
}

func (a *Action) assertLoaded() {
	if a.def == nil {
		panic("action data is not available, call \"EnsureLoaded\" method first to load the data")
	}
}

// ActionDef returns action definition with replaced variables.
func (a *Action) ActionDef() *DefAction {
	a.assertLoaded()
	return a.def.Action
}

// RuntimeDef returns runtime definition.
func (a *Action) RuntimeDef() *DefRuntime {
	a.assertLoaded()
	return a.def.Runtime
}

// ImageBuildInfo implements [ImageBuildResolver].
func (a *Action) ImageBuildInfo(image string) *types.BuildDefinition {
	return a.RuntimeDef().Container.Build.ImageBuildInfo(image, a.Dir())
}

// SetInput saves arguments and options for later processing in run, templates, etc.
func (a *Action) SetInput(input *Input) (err error) {
	def, err := a.Raw()
	if err != nil {
		return err
	}

	// Process arguments.
	err = a.processInputParams(def.Action.Arguments, input.ArgsNamed())
	if err != nil {
		return err
	}

	// Process options.
	err = a.processInputParams(def.Action.Options, input.OptsAll())
	if err != nil {
		return err
	}

	// Validate the new input.
	if err = a.ValidateInput(input); err != nil {
		return err
	}

	a.input = input
	// Reset to load the action file again with new replacements.
	a.Reset()
	return a.EnsureLoaded()
}

func (a *Action) processInputParams(def ParametersList, inp InputParams) error {
	for _, p := range def {
		if _, ok := inp[p.Name]; !ok {
			continue
		}

		value := inp[p.Name]
		toApply := p.Process

		value, err := a.processValue(value, p.Type, toApply)
		if err != nil {
			return err
		}
		// Replace the value.
		// Check for nil not to override the default value.
		if value != nil {
			inp[p.Name] = value
		}
	}

	return nil
}

func (a *Action) processValue(v any, vtype jsonschema.Type, applyProc []DefValueProcessor) (any, error) {
	res := v
	processors := a.GetProcessors()

	for _, procDef := range applyProc {
		proc, ok := processors[procDef.ID]
		if !ok {
			return v, fmt.Errorf(errTplNonExistProcessor, procDef.ID)
		}

		if !proc.IsApplicable(vtype) {
			return v, fmt.Errorf(errTplNotApplicableProcessor, vtype)
		}

		processedValue, err := proc.Execute(res, procDef.Options)
		if err != nil {
			return v, err
		}

		res = processedValue
	}
	// Cast to []any slice because jsonschema validator supports only this type.
	if vtype == jsonschema.Array {
		res = CastSliceTypedToAny(res)
	}

	return res, nil
}

// ValidateInput validates action input.
func (a *Action) ValidateInput(input *Input) error {
	if input.IsValidated() {
		return nil
	}
	argsDefLen := len(a.ActionDef().Arguments)
	argsPosLen := len(input.ArgsPositional())
	if argsPosLen > argsDefLen {
		return fmt.Errorf("accepts %d arg(s), received %d", argsDefLen, argsPosLen)
	}
	return validateJSONSchema(a, input)
}

// Execute runs action in the specified environment.
func (a *Action) Execute(ctx context.Context) error {
	// @todo maybe it shouldn't be here.
	if a.runtime == nil {
		panic("runtime is not set, call SetRuntime first")
	}
	defer a.runtime.Close()
	if err := a.runtime.Init(ctx, a); err != nil {
		return err
	}
	return a.runtime.Execute(ctx, a)
}
