package action

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/launchrctl/launchr/internal/launchr"
	"github.com/launchrctl/launchr/pkg/driver"
	"github.com/launchrctl/launchr/pkg/jsonschema"
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
	fs     DiscoveryFS // fs is a filesystem where the action was discovered. May be nil if created manually.
	fpath  string      // fpath is a path to action definition file.
	def    *Definition // def is an action definition. Loaded by [Loader], may be nil when not initialized.
	defRaw *Definition // defRaw is a raw action definition. Loaded by [Loader], may be nil when not initialized.

	runtime    Runtime                   // runtime is the [Runtime] to execute the action.
	input      *Input                    // input is a storage for arguments and options used in runtime.
	processors map[string]ValueProcessor // processors are [ValueProcessor] for manipulating input.
}

// New creates a new action.
func New(idp IDProvider, l Loader, fsys DiscoveryFS, fpath string) *Action {
	// We don't define ID here because we use [Action] object for
	// context creation to calculate ID later.
	a := &Action{
		loader: l,
		fs:     fsys,
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
	return New(StringID(id), &YamlLoader{Bytes: b}, NewDiscoveryFS(nil, ""), "")
}

// NewYAMLFromFS creates an action from the given filesystem.
// The filesystem must have action.yaml in the root.
func NewYAMLFromFS(id string, fsys fs.FS) (*Action, error) {
	d := NewDiscovery(
		NewDiscoveryFS(fsys, ""),
		YamlDiscoveryStrategy{TargetRgx: rgxYamlRootFile},
	)
	d.SetActionIDProvider(StringID(id))
	discovered, err := d.Discover(context.Background())
	if err != nil {
		// Normally error doesn't happen. Or we didn't check all cases.
		return nil, err
	}
	if len(discovered) > 0 {
		return discovered[0], nil
	}
	return nil, fmt.Errorf("no actions found in the given filesystem")
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
		fs:     a.fs,
		fpath:  a.fpath,
	}
	if a.runtime != nil {
		c.runtime = a.runtime.Clone()
	}
	return c
}

// SetProcessors sets the value processors for an [Action].
func (a *Action) SetProcessors(list map[string]ValueProcessor) error {
	def := a.ActionDef()
	for _, params := range []ParametersList{def.Arguments, def.Options} {
		for _, p := range params {
			err := p.InitProcessors(list)
			if err != nil {
				return err
			}
		}
	}

	return nil
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
	a.wd = launchr.MustAbs(wd)
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

// syncToDisk copies action fs to disk if it's virtual like embed.
// After finish the result is cached per action run.
func (a *Action) syncToDisk() (err error) {
	// If there is no fs or it's already on the disk.
	if a.fs.fs == nil || a.fs.Realpath() != "" {
		return
	}
	// Export to a temporary path.
	// Make sure the path doesn't have semicolons, because Docker bind doesn't like it.
	tmpDirName := strings.Replace(a.ID, ":", "_", -1)
	tmpDir, err := launchr.MkdirTemp(tmpDirName)
	if err != nil {
		return
	}
	fsys, err := fs.Sub(a.fs.fs, a.Dir())
	if err != nil {
		return
	}
	// Copy from memory to the disk.
	err = os.CopyFS(tmpDir, fsys)
	if err != nil {
		return
	}
	// Set a new filesystem to a cached path.
	a.fs = NewDiscoveryFS(os.DirFS(tmpDir), a.fs.wd)
	return
}

// Filepath returns action file path.
func (a *Action) Filepath() string {
	return filepath.Join(a.fs.Realpath(), a.fpath)
}

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
	// Load raw definition as well.
	_, err = a.Raw()
	if err != nil {
		return err
	}
	// Load with replacements.
	a.def, err = a.loader.Load(LoadContext{Action: a})
	return err
}

// ActionDef returns action definition.
func (a *Action) ActionDef() *DefAction {
	raw, err := a.Raw()
	if err != nil {
		// All discovered actions are checked for error.
		// It means that normally by this time you shouldn't receive this panic.
		// Please, review your code.
		// The error may occur if there is a new flow for action.
		// You may need to manually check the error of Action.Raw() or Action.EnsureLoaded().
		panic(fmt.Errorf("load error must be checked first: %w", err))
	}
	return raw.Action
}

// RuntimeDef returns runtime definition with replaced variables.
func (a *Action) RuntimeDef() *DefRuntime {
	err := a.EnsureLoaded()
	if err != nil {
		// The error may appear if the action is incorrectly defined.
		// Normally EnsureLoaded is called when user input is set and variables are recalculated.
		// It means that by this time you shouldn't receive this panic.
		// Please, review your code.
		// Call SetInput or EnsureLoaded to check for the error before accessing this data.
		panic(fmt.Errorf("load error must be checked first: %w", err))
	}
	return a.def.Runtime
}

// ImageBuildInfo implements [ImageBuildResolver].
func (a *Action) ImageBuildInfo(image string) *driver.BuildDefinition {
	return a.RuntimeDef().Container.Build.ImageBuildInfo(image, a.Dir())
}

// SetInput saves arguments and options for later processing in run, templates, etc.
func (a *Action) SetInput(input *Input) (err error) {
	def := a.ActionDef()

	// Process arguments.
	err = a.processInputParams(def.Arguments, input.Args(), input.ArgsChanged())
	if err != nil {
		return err
	}

	// Process options.
	err = a.processInputParams(def.Options, input.Opts(), input.OptsChanged())
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

func (a *Action) processInputParams(def ParametersList, inp InputParams, changed InputParams) error {
	var err error
	for _, p := range def {
		_, isChanged := changed[p.Name]
		res := inp[p.Name]
		for i, procDef := range p.Process {
			handler := p.processors[i]
			res, err = handler(res, ValueProcessorContext{
				ValOrig:   inp[p.Name],
				IsChanged: isChanged,
				DefParam:  p,
				Action:    a,
			})
			if err != nil {
				return ErrValueProcessorHandler{
					Processor: procDef.ID,
					Param:     p.Name,
					Err:       err,
				}
			}
		}
		// Cast to []any slice because jsonschema validator supports only this type.
		if p.Type == jsonschema.Array {
			res = CastSliceTypedToAny(res)
		}
		// If the value was changed, we can safely override the value.
		// If the value was not changed and processed is nil, do not add it.
		if isChanged || res != nil {
			inp[p.Name] = res
		}
	}

	return nil
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
	err := validateJSONSchema(a, input)
	if err != nil {
		return err
	}
	input.SetValidated(true)
	return nil
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
