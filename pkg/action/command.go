package action

import (
	"path/filepath"
)

// Command is an action with a contextual name, working directory path
// and a runtime context such as input arguments and options.
type Command struct {
	WorkingDir  string
	Filepath    string
	CommandName string

	Loader Loader
	action *Action

	InputArgs    map[string]string
	InputOptions map[string]interface{}
}

func (cmd *Command) loadAction() (*Action, error) {
	if cmd.action != nil {
		return cmd.action, nil
	}
	var err error
	cmd.action, err = cmd.Loader.Load()
	return cmd.action, err
}

// Compile loads an action with replaced arguments and options.
func (cmd *Command) Compile() error {
	cmd.action = nil
	_, err := cmd.loadAction()
	return err
}

// Action returns action with replaces variables.
func (cmd *Command) Action() *ActionConfig {
	if cmd.action == nil {
		panic("action data is not available, call \"Compile\" method first to load the data")
	}
	return cmd.action.Action
}

// Dir returns an action file directory.
func (cmd *Command) Dir() string {
	return filepath.Dir(filepath.Join(cmd.WorkingDir, cmd.Filepath))
}

// SetArgsInput saves passed cobra arguments
// for later processing in run, templates, etc.
func (cmd *Command) SetArgsInput(args []string) {
	// Load raw action to retrieve arguments definition.
	cmd.InputArgs = nil
	err := cmd.Compile()
	if err != nil {
		// Just return. The error will pop up on the new compile.
		return
	}
	mapped := make(map[string]string, len(args))
	argsDef := cmd.Action().Arguments
	for i, a := range args {
		if i < len(argsDef) {
			mapped[argsDef[i].Name] = a
		}
	}
	cmd.InputArgs = mapped
	cmd.action = nil // Reset action to load action again with new replacements.
}

// SetOptsInput saves passed cobra flags
// for later processing in run, templates, etc.
func (cmd *Command) SetOptsInput(options map[string]interface{}) {
	cmd.InputOptions = options
	cmd.action = nil // Reset action to load action again with new replacement.
}

// ValidateInput validates arguments and options according to
// a specified json schema in action definition.
func (cmd *Command) ValidateInput() error {
	// @todo implement json schema validation
	// @todo generate json schema
	return nil
}

// ActionRaw returns encoded action.
func (cmd *Command) ActionRaw() ([]byte, error) {
	return cmd.Loader.Content()
}
