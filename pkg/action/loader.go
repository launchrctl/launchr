package action

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"regexp"
	"strings"
	"sync"
	"text/template"

	"github.com/a8m/envsubst"
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

type yamlFileLoader struct {
	processor LoadProcessor
	raw       *Definition
	cached    []byte
	open      func() (fs.File, error)
	mx        sync.Mutex
}

func (l *yamlFileLoader) Content() ([]byte, error) {
	l.mx.Lock()
	defer l.mx.Unlock()
	// @todo unload unused, maybe manager must do it.
	var err error
	if l.cached != nil {
		return l.cached, nil
	}
	f, err := l.open()
	if err != nil {
		return nil, err
	}
	defer f.Close()
	l.cached, err = io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	return l.cached, nil
}

func (l *yamlFileLoader) LoadRaw() (*Definition, error) {
	var err error
	buf, err := l.Content()
	if err != nil {
		return nil, err
	}
	l.mx.Lock()
	defer l.mx.Unlock()
	if l.raw == nil {
		l.raw, err = CreateFromYamlTpl(buf)
		if err != nil {
			return nil, err
		}
	}
	return l.raw, err
}

func (l *yamlFileLoader) Load(ctx LoadContext) (res *Definition, err error) {
	// Open a file and cache content for future reads.
	c, err := l.Content()
	if err != nil {
		return nil, err
	}
	buf := make([]byte, len(c))
	copy(buf, c)
	buf, err = l.processor.Process(ctx, buf)
	if err != nil {
		return nil, err
	}
	r := bytes.NewReader(buf)
	res, err = CreateFromYaml(r)
	if err != nil {
		return nil, err
	}
	return res, err
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

type escapeYamlTplCommentsProcessor struct{}

func (p escapeYamlTplCommentsProcessor) Process(_ LoadContext, b []byte) ([]byte, error) {
	// Read by line.
	scanner := bufio.NewScanner(bytes.NewBuffer(b))
	res := make([]byte, 0, len(b))
	for scanner.Scan() {
		l := scanner.Bytes()
		if i := bytes.IndexByte(l, '#'); i != -1 {
			// Check the comment symbol is not inside a string.
			// Multiline strings are not supported for now.
			if !(bytes.LastIndexByte(l[:i], '"') != -1 && bytes.IndexByte(l[i:], '"') != -1 ||
				bytes.LastIndexByte(l[:i], '\'') != -1 && bytes.IndexByte(l[i:], '\'') != -1) {
				// Strip data after comment symbol.
				l = l[:i]
			}
		}
		// Collect the modified lines.
		res = append(res, l...)
		res = append(res, '\n')
	}
	return res, nil
}

type envProcessor struct{}

func (p envProcessor) Process(_ LoadContext, b []byte) ([]byte, error) {
	return envsubst.Bytes(b)
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
	def, err := ctx.Action.Loader.LoadRaw()
	if err != nil {
		return nil, err
	}
	// Collect template variables.
	data := ConvertInputToTplVars(a.GetInput(), def.Action)
	addPredefinedVariables(data)

	// Parse action without variables to validate
	tpl := template.New(a.ID)
	_, err = tpl.Parse(string(b))
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
func ConvertInputToTplVars(input Input, ac *DefAction) map[string]interface{} {
	values := make(map[string]interface{}, len(input.Args)+len(input.Opts))
	// Collect argument values.
	for _, arg := range ac.Arguments {
		key := arg.Name
		values[key] = ""
		values[replDashes.Replace(key)] = ""
		if v, ok := input.Args[arg.Name]; ok {
			// Allow usage of dashed variable names like "my-name" by replacing dashes to underscores.
			values[key] = v
			values[replDashes.Replace(key)] = v
		}
	}

	// Collect options values.
	for _, o := range ac.Options {
		key := o.Name
		// Set value default or input option.
		values[key] = o.Default
		values[replDashes.Replace(key)] = o.Default
		if v, ok := input.Opts[o.Name]; ok {
			// Allow usage of dashed variable names like "my-name" by replacing dashes to underscores.
			values[key] = v
			values[replDashes.Replace(key)] = v
		}
	}

	// @todo consider boolean, it's strange in output - "true/false"
	// @todo handle array options

	return values
}

func addPredefinedVariables(data map[string]interface{}) {
	cuser := getCurrentUser()
	// Set zeros for running in environments like Windows
	data["current_uid"] = 0
	data["current_gid"] = 0
	if cuser != "" {
		s := strings.Split(cuser, ":")
		data["current_uid"] = s[0]
		data["current_gid"] = s[1]
	}
}
