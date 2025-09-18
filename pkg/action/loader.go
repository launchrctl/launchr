package action

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"
	"text/template"

	"github.com/launchrctl/launchr/internal/launchr"
)

// Loader is an interface for loading an action file.
type Loader interface {
	// Content returns the raw file content.
	Content() ([]byte, error)
	// Load parses Content to a Definition with substituted values.
	Load(*LoadContext) (*Definition, error)
}

// LoadContext stores relevant and isolated data needed for processors.
type LoadContext struct {
	Action   *Action
	Services launchr.ServiceManager

	tplVars    map[string]any
	tplFuncMap template.FuncMap
}

func (ctx *LoadContext) getActionTemplateProcessors() TemplateProcessors {
	if ctx.Services == nil {
		return NewTemplateProcessors()
	}
	var tp TemplateProcessors
	ctx.Services.Get(&tp)
	return tp
}

func (ctx *LoadContext) getTemplateFuncMap() template.FuncMap {
	if ctx.tplFuncMap == nil {
		procs := ctx.getActionTemplateProcessors()
		ctx.tplFuncMap = procs.GetTemplateFuncMap(TemplateFuncContext{Action: ctx.Action})
	}
	return ctx.tplFuncMap
}

func (ctx *LoadContext) getTemplateData() map[string]any {
	if ctx.tplVars == nil {
		def := ctx.Action.ActionDef()
		// Collect template variables.
		ctx.tplVars = convertInputToTplVars(ctx.Action.Input(), def)
		addPredefinedVariables(ctx.tplVars, ctx.Action)
	}
	return ctx.tplVars
}

// LoadProcessor is an interface for processing input on load.
type LoadProcessor interface {
	// Process gets an input action file data and returns a processed result.
	Process(*LoadContext, string) (string, error)
}

type pipeProcessor struct {
	p []LoadProcessor
}

// NewPipeProcessor creates a new processor containing several processors that handles input consequentially.
func NewPipeProcessor(p ...LoadProcessor) LoadProcessor {
	return &pipeProcessor{p: p}
}

func (p *pipeProcessor) Process(ctx *LoadContext, s string) (string, error) {
	var err error
	for _, proc := range p.p {
		s, err = proc.Process(ctx, s)
		if err != nil {
			return s, err
		}
	}
	return s, nil
}

type envProcessor struct{}

func (p envProcessor) Process(ctx *LoadContext, s string) (string, error) {
	if ctx.Action == nil {
		panic("envProcessor received nil LoadContext.Action")
	}
	if !strings.Contains(s, "$") {
		return s, nil
	}
	pv := newPredefinedVars(ctx.Action)
	getenv := func(key string) string {
		v, ok := pv.getenv(key)
		if ok {
			return v
		}
		return launchr.Getenv(key)
	}
	return os.Expand(s, getenv), nil
}

type inputProcessor struct{}

var rgxTplVar = regexp.MustCompile(`{{.*?\.([a-zA-Z][a-zA-Z0-9_]*).*?}}`)

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

func (p inputProcessor) Process(ctx *LoadContext, s string) (string, error) {
	if ctx.Action == nil {
		panic("inputProcessor received nil LoadContext.Action")
	}
	if !strings.Contains(s, "{{") {
		return s, nil
	}

	// Collect template variables.
	data := ctx.getTemplateData()

	// Parse action yaml.
	tpl := template.New(s).Funcs(ctx.getTemplateFuncMap())
	_, err := tpl.Parse(s)

	// Check if variables have dashes to show the error properly.
	err = checkDashErr(err, data)
	if err != nil {
		return s, err
	}

	// Execute template.
	buf := bytes.NewBuffer(make([]byte, 0, len(s)))
	err = tpl.Execute(buf, data)
	if err != nil {
		return s, err
	}

	// Find if some vars were used but not defined in arguments or options.
	res := buf.String()
	err = findMissingVars(s, res, data)
	if err != nil {
		return s, err
	}

	return res, nil
}

// convertInputToTplVars creates a map with input variables suitable for template engine.
func convertInputToTplVars(input *Input, ac *DefAction) map[string]any {
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
		dval := pdef.Default
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
	// TODO: Deprecated, use env variables.
	pv := newPredefinedVars(a)
	for k, v := range pv.templateData() {
		data[k] = v
	}
}

func checkDashErr(err error, data map[string]any) error {
	if err == nil {
		return nil
	}
	// Check if variables have dashes to show the error properly.
	hasDash := false
	for k := range data {
		if strings.Contains(k, "-") {
			hasDash = true
			break
		}
	}
	if hasDash && strings.Contains(err.Error(), "bad character U+002D '-'") {
		return fmt.Errorf(`unexpected '-' symbol in a template variable. 
Action definition is correct, but dashes are not allowed in templates, replace "-" with "_" in {{ }} blocks`)
	}
	return err
}

func findMissingVars(orig, repl string, data map[string]any) error {
	miss := make(map[string]struct{})
	if !strings.Contains(repl, "<no value>") {
		return nil
	}
	matches := rgxTplVar.FindAllStringSubmatch(orig, -1)
	for _, m := range matches {
		k := m[1]
		if _, ok := data[k]; !ok {
			miss[k] = struct{}{}
		}
	}
	// If we don't have parameter names here, it means that all parameters are defined but the values were nil.
	// It's ok, users will be able to identify missing parameters.
	if len(miss) != 0 {
		return errMissingVar{miss}
	}
	return nil
}

// processStructStringsInPlace walks over a struct recursively and processes string fields in-place using the provided processor function.
func processStructStringsInPlace(v any, processor func(string) (string, error)) error {
	return processStructValueInPlace(reflect.ValueOf(v), processor)
}

func processStructValueInPlace(v reflect.Value, processor func(string) (string, error)) error {
	// Handle pointers and interfaces
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return nil
		}
		return processStructValueInPlace(v.Elem(), processor)
	}

	if v.Kind() == reflect.Interface {
		if v.IsNil() {
			return nil
		}
		return processStructValueInPlace(v.Elem(), processor)
	}

	switch v.Kind() {
	case reflect.String:
		return processStringValueInPlace(v, processor)
	case reflect.Struct:
		return processStructFieldsInPlace(v, processor)
	case reflect.Slice, reflect.Array:
		return processSliceOrArrayInPlace(v, processor)
	case reflect.Map:
		return processMapValueInPlace(v, processor)
	default:
		return nil
	}
}

func processStringValueInPlace(v reflect.Value, processor func(string) (string, error)) error {
	if !v.CanSet() {
		return nil
	}

	processed, err := processor(v.String())
	if err != nil {
		return err
	}

	// Only process basic string types directly
	if v.Type() == reflect.TypeOf("") {
		v.SetString(processed)
		return nil
	}

	// For custom string types, try to convert and set
	newVal := reflect.ValueOf(processed)
	if newVal.Type().ConvertibleTo(v.Type()) {
		v.Set(newVal.Convert(v.Type()))
	}

	return nil
}

func processStructFieldsInPlace(v reflect.Value, processor func(string) (string, error)) error {
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		structField := v.Type().Field(i)

		// Skip unexported fields
		if !structField.IsExported() {
			continue
		}

		if field.CanInterface() {
			if err := processStructValueInPlace(field, processor); err != nil {
				return err
			}
		}
	}

	return nil
}

func processSliceOrArrayInPlace(v reflect.Value, processor func(string) (string, error)) error {
	for i := 0; i < v.Len(); i++ {
		if err := processStructValueInPlace(v.Index(i), processor); err != nil {
			return err
		}
	}

	return nil
}

func processMapValueInPlace(v reflect.Value, processor func(string) (string, error)) error {
	// For maps, we need to handle key processing differently since map keys are not addressable
	keysToUpdate := make(map[reflect.Value]reflect.Value)

	for _, key := range v.MapKeys() {
		value := v.MapIndex(key)

		// Process the value in-place if possible
		if err := processStructValueInPlace(value, processor); err != nil {
			return err
		}

		// Check if key needs processing (only for string keys)
		if key.Kind() == reflect.String {
			processed, err := processor(key.String())
			if err != nil {
				return err
			}

			// If the key changed, we need to update the map
			if processed != key.String() {
				var newKey reflect.Value
				if key.Type() == reflect.TypeOf("") {
					newKey = reflect.ValueOf(processed)
				} else {
					newKeyVal := reflect.ValueOf(processed)
					if newKeyVal.Type().ConvertibleTo(key.Type()) {
						newKey = newKeyVal.Convert(key.Type())
					} else {
						continue // Skip if not convertible
					}
				}
				keysToUpdate[key] = newKey
			}
		}
	}

	// Update map keys that changed
	for oldKey, newKey := range keysToUpdate {
		value := v.MapIndex(oldKey)
		v.SetMapIndex(oldKey, reflect.Value{}) // Delete old key
		v.SetMapIndex(newKey, value)           // Set with new key
	}

	return nil
}
