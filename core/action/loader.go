package action

import (
	"bytes"
	"fmt"
	"regexp"
	"text/template"

	"github.com/a8m/envsubst"
)

// Loader is an interface for loading a config file.
type Loader interface {
	Content() ([]byte, error)
	Load() (*Config, error)
	LoadRaw() (*Config, error)
}

// LoadProcessor is an interface for processing input on load.
type LoadProcessor interface {
	Process([]byte) ([]byte, error)
}

type pipeProcessor struct {
	p []LoadProcessor
}

// NewPipeProcessor creates a new processor containing several processors that handles input consequentially.
func NewPipeProcessor(p ...LoadProcessor) LoadProcessor {
	return &pipeProcessor{p: p}
}

func (p *pipeProcessor) Process(b []byte) ([]byte, error) {
	var err error
	for _, proc := range p.p {
		b, err = proc.Process(b)
		if err != nil {
			return b, err
		}
	}
	return b, nil
}

type envProcessor struct{}

func (p *envProcessor) Process(b []byte) ([]byte, error) {
	return envsubst.Bytes(b)
}

type inputProcessor struct {
	cmd *Command
}

var rgxTplVar = regexp.MustCompile(`{{.*?\.(\S+).*?}}`)

type errMissingVar struct {
	vars map[string]struct{}
}

// Error implements error interface.
func (err errMissingVar) Error() string {
	f := make([]string, 0, len(err.vars))
	for k := range err.vars {
		f = append(f, k)
	}
	return fmt.Sprintf("the following variables were used but never defined: %v", f)
}

func (p *inputProcessor) Process(b []byte) ([]byte, error) {
	conf, err := p.cmd.Loader.LoadRaw()
	if err != nil {
		return nil, err
	}
	data := ConvertInputToStruct(p.cmd, conf)
	tpl := template.New(p.cmd.CommandName)
	_, err = tpl.Parse(string(b))
	if err != nil {
		return nil, err
	}

	buf := bytes.NewBuffer(make([]byte, 0, len(b)))
	err = tpl.Execute(buf, data)
	if err != nil {
		return nil, err
	}

	// Find if some vars were used but not defined.
	miss := make(map[string]struct{})
	res := buf.Bytes()
	if bytes.Contains(res, []byte("<no value>")) {
		matches := rgxTplVar.FindAllSubmatch(b, -1)
		for _, m := range matches {
			k := string(m[1])
			if _, ok := data[k]; !ok {
				miss[k] = struct{}{}
			}
		}
		return nil, errMissingVar{miss}
	}

	return res, nil
}

// ConvertInputToStruct creates an arbitrary struct from input variables.
func ConvertInputToStruct(cmd *Command, conf *Config) map[string]interface{} {
	a := conf.Action
	cnt := len(cmd.InputArgs) + len(cmd.InputOptions)
	values := make(map[string]interface{}, cnt)

	// Collect argument values.
	for _, arg := range a.Arguments {
		key := arg.Name
		values[key] = ""
		if v, ok := cmd.InputArgs[arg.Name]; ok {
			values[key] = v
		}
	}

	// Collect options values.
	for _, o := range a.Options {
		key := o.Name
		// Set value default or input option.
		values[key] = o.Default
		if v, ok := cmd.InputOptions[o.Name]; ok {
			values[key] = v
		}
	}

	// @todo consider boolean, it's strange in output - "true/false"
	// @todo handle array options

	return values
}
