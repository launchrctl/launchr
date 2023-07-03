package action

import (
	"path"
)

// Command holds a definition of a discovered action file.
type Command struct {
	WorkingDir  string
	Filepath    string
	CommandName string

	Loader Loader
	cfg    *Config

	InputArgs    map[string]string
	InputOptions map[string]interface{}
}

func (cmd *Command) loadConfig() (*Config, error) {
	if cmd.cfg != nil {
		return cmd.cfg, nil
	}
	var err error
	cmd.cfg, err = cmd.Loader.Load()
	return cmd.cfg, err
}

// Compile load config with replaced arguments and options.
func (cmd *Command) Compile() error {
	cmd.cfg = nil
	_, err := cmd.loadConfig()
	return err
}

// Action returns action with replaces variables.
func (cmd *Command) Action() *Action {
	if cmd.cfg == nil {
		panic("action data is not available, call \"Compile\" method first to load the data")
	}
	return cmd.cfg.Action
}

// Dir returns an action file directory.
func (cmd *Command) Dir() string {
	return path.Dir(path.Join(cmd.WorkingDir, cmd.Filepath))
}

// SetArgsInput saves passed cobra arguments
// for later processing in run, templates, etc.
func (cmd *Command) SetArgsInput(args []string) {
	// Load raw config.
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
	cmd.cfg = nil // Reset cfg to load action again with new replacements.
}

// SetOptsInput saves passed cobra flags
// for later processing in run, templates, etc.
func (cmd *Command) SetOptsInput(options map[string]interface{}) {
	cmd.InputOptions = options
	cmd.cfg = nil // Reset cfg to load action again with new replacement.
}

// ValidateInput validates arguments and options according to
// a specified json schema in action definition.
func (cmd *Command) ValidateInput() error {
	// @todo implement json schema validation
	// @todo generate json schema
	return nil
}
