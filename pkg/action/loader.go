package action

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"
	"syscall"
	"text/template"
)

// Loader is an interface for loading an action file.
type Loader interface {
	// Content returns the raw file content.
	Content() ([]byte, error)
	// Load parses Content to a Definition with substituted values.
	Load(LoadContext) (*Definition, error)
	// LoadRaw parses Content to a Definition raw values. Template strings are escaped.
	LoadRaw() (*Definition, error)
}

// LoadContext stores relevant and isolated data needed for processors.
type LoadContext struct {
	Action *Action
}

// LoadProcessor is an interface for processing input on load.
type LoadProcessor interface {
	// Process gets an input action file data and returns a processed result.
	Process(LoadContext, []byte) ([]byte, error)
}

type pipeProcessor struct {
	p []LoadProcessor
}

// NewPipeProcessor creates a new processor containing several processors that handles input consequentially.
func NewPipeProcessor(p ...LoadProcessor) LoadProcessor {
	return &pipeProcessor{p: p}
}

func (p *pipeProcessor) Process(ctx LoadContext, b []byte) ([]byte, error) {
	var err error
	for _, proc := range p.p {
		b, err = proc.Process(ctx, b)
		if err != nil {
			return b, err
		}
	}
	return b, nil
}

type envProcessor struct{}

func (p envProcessor) Process(_ LoadContext, b []byte) ([]byte, error) {
	s := os.Expand(string(b), getenv)
	return []byte(s), nil
}

func getenv(key string) string {
	if key == "$" {
		return "$"
	}
	// Replace all subexpressions.
	if strings.Contains(key, "$") {
		key = os.Expand(key, getenv)
	}
	// @todo implement ${var-$DEFAULT}, ${var:-$DEFAULT}, ${var+$DEFAULT}, ${var:+$DEFAULT},
	v, _ := syscall.Getenv(key)
	return v
}

type inputProcessor struct{}

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

func (p inputProcessor) Process(ctx LoadContext, b []byte) ([]byte, error) {
	if ctx.Action == nil {
		return b, nil
	}
	a := ctx.Action
	def := ctx.Action.ActionDef()
	// Collect template variables.
	data := ConvertInputToTplVars(a.Input(), def)
	addPredefinedVariables(data, a)

	// Parse action without variables to validate
	tpl := template.New(a.ID)
	_, err := tpl.Parse(string(b))
	if err != nil {
		// Check if variables have dashes to show the error properly.
		hasDash := false
		for k := range data {
			if strings.Contains(k, "-") {
				hasDash = true
				break
			}
		}
		if hasDash && strings.Contains(err.Error(), "bad character U+002D '-'") {
			return nil, fmt.Errorf(`unexpected '-' symbol in a template variable. 
Action definition is correct, but dashes are not allowed in templates, replace "-" with "_" in {{ }} blocks`)
		}
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

// ConvertInputToTplVars creates a map with input variables suitable for template engine.
func ConvertInputToTplVars(input *Input, ac *DefAction) map[string]any {
	args := input.Args()
	opts := input.Opts()
	values := make(map[string]any, len(args)+len(opts))

	// Collect arguments and options values.
	collectInputVars(values, args, ac.Arguments)
	collectInputVars(values, opts, ac.Options)

	// @todo consider boolean, it's strange in output - "true/false"
	// @todo handle array options

	return values
}

func collectInputVars(values map[string]any, params InputParams, def ParametersList) {
	for _, pdef := range def {
		key := pdef.Name
		// Set value: default or input parameter.
		dval := fmt.Sprintf("%v", pdef.Default)
		values[key] = dval
		values[replDashes.Replace(key)] = dval
		if v, ok := params[pdef.Name]; ok {
			// Allow usage of dashed variable names like "my-name" by replacing dashes to underscores.
			values[key] = v
			values[replDashes.Replace(key)] = v
		}
	}
}

func addPredefinedVariables(data map[string]any, a *Action) {
	cuser := getCurrentUser()
	// Set zeros for running in environments like Windows
	data["current_uid"] = 0
	data["current_gid"] = 0
	if cuser != "" {
		s := strings.Split(cuser, ":")
		data["current_uid"] = s[0]
		data["current_gid"] = s[1]
	}
	data["current_working_dir"] = a.wd         // app working directory
	data["actions_base_dir"] = a.fs.Realpath() // root directory where the action was found
	data["action_dir"] = a.Dir()               // directory of action file
}
