package action

import (
	"bytes"
	"context"
	"encoding/json"
	jsvalidate "github.com/santhosh-tekuri/jsonschema/v5"
	"path/filepath"

	"github.com/launchrctl/launchr/pkg/types"
)

type FileAction struct {
	baseAction
	Loader Loader // Loader is a function to load action definition. Helpful to reload with replaced variables.

	// wd is a working directory set from app level.
	// Usually current working directory, but may be overridden by a plugin.
	wd    string
	fsdir string // fsdir is a base directory where the action was discovered (for better ID naming).
	fpath string // fpath is a path to action definition file.
}

// SetInput saves arguments and options for later processing in run, templates, etc.
func (a *FileAction) SetInput(input Input) (err error) {
	if err = a.EnsureLoaded(); err != nil {
		return err
	}
	// @todo disabled for now until fully tested.
	//if err = a.ValidateInput(input); err != nil {
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

// ValidateInput validates arguments and options according to
// a specified json schema in action definition.
// @todo move to jsonschema
func (a *FileAction) ValidateInput(inp Input) error {
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

// Execute runs action in the specified environment.
func (a *FileAction) Execute(ctx context.Context) error {
	var act Action = a
	return a.baseAction.execute(ctx, act)
}

// Reset unsets loaded action to force reload.
func (a *FileAction) Reset() { a.def = nil }

// WorkDir returns action working directory.
func (a *FileAction) WorkDir() string {
	if a.def != nil && a.def.WD != "" {
		wd, err := filepath.Abs(filepath.Clean(a.def.WD))
		if err == nil {
			return wd
		}
	}
	return a.wd
}

// Filepath returns action file path.
func (a *FileAction) Filepath() string { return a.fpath }

// Dir returns an action file directory.
func (a *FileAction) Dir() string { return filepath.Dir(a.Filepath()) }

// DefinitionEncoded returns encoded action file content.
func (a *FileAction) DefinitionEncoded() ([]byte, error) { return a.Loader.Content() }

// EnsureLoaded loads an action file with replaced arguments and options.
func (a *FileAction) EnsureLoaded() (err error) {
	if a.def != nil {
		return err
	}
	a.def, err = a.Loader.Load(LoadContext{Action: a})
	return err
}

// ImageBuildInfo implements ImageBuildResolver.
func (a *FileAction) ImageBuildInfo(image string) *types.BuildDefinition {
	return a.ActionDef().Build.ImageBuildInfo(image, a.Dir())
}

// Clone returns a copy of an action.
func (a *FileAction) Clone() Action {
	if a == nil {
		return nil
	}
	c := &FileAction{
		baseAction: baseAction{
			ID: a.ID,
		},
		wd:     a.wd,
		fsdir:  a.fsdir,
		fpath:  a.fpath,
		Loader: a.Loader,
	}
	return c
}

// NewFileAction creates a new action.
func NewFileAction(id, wd, fsdir, fpath string) *FileAction {
	return &FileAction{
		baseAction: baseAction{
			ID: id,
		},
		wd:    wd,
		fsdir: fsdir,
		fpath: fpath,
	}
}
