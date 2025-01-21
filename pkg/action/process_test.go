package action

import (
	"fmt"
	"strings"
	"testing"

	"github.com/launchrctl/launchr/pkg/jsonschema"
)

const actionProcessWithDefault = `
runtime: plugin
action:
  title: Title
  arguments:
    - name: arg1
      default: "arg_default"
      process:
        - processor: test.defaultVal
        - processor: test.replace
          options:
            old: A
            new: B
        - processor: test.replace
          options:
            old: C
            new: D
  options:
    - name: opt1
      default: "opt_default"
      process:
        - processor: test.defaultVal
        - processor: test.replace
          options:
            old: A
            new: B
        - processor: test.replace
          options:
            old: C
            new: D
`

const actionProcessNoDefault = `
runtime: plugin
action:
  title: Title
  arguments:
    - name: arg1
      process:
        - processor: test.defaultVal
        - processor: test.replace
          options:
            old: A
            new: B
        - processor: test.replace
          options:
            old: C
            new: D
  options:
    - name: opt1
      process:
        - processor: test.defaultVal
        - processor: test.replace
          options:
            old: A
            new: B
        - processor: test.replace
          options:
            old: C
            new: D
`

const actionProcessBroken = `
runtime: plugin
action:
  title: Title
  arguments:
    - name: arg1
      process:
        - processor: test.broken
`

const actionProcessWrongOptions = `
runtime: plugin
action:
  title: Title
  arguments:
    - name: arg1
      process:
        - processor: test.replace
          options:
            old: [1, 2, 3]
            new:
              obj: str
`

const actionProcessReturnErr = `
runtime: plugin
action:
  title: Title
  arguments:
    - name: arg1
      process:
        - processor: test.error
`

const actionProcessUnsupType = `
runtime: plugin
action:
  title: Title
  arguments:
    - name: arg1
      type: integer
      process:
        - processor: test.replace
          options:
            old: A
            new: B
`

const actionProcessArrayType = `
runtime: plugin
action:
  title: Title
  arguments:
    - name: arg1
      type: array
      process:
        - processor: test.defaultVal
`

type procTestReplaceOptions struct {
	O string `yaml:"old"`
	N string `yaml:"new"`
}

func (o *procTestReplaceOptions) Validate() error {
	if o.O == "" {
		return fmt.Errorf("parameter old required")
	}
	return nil
}

func addTestValueProcessors(am Manager) {
	procDefVal := GenericValueProcessor[*ValueProcessorOptionsEmpty]{
		Fn: func(v any, _ *ValueProcessorOptionsEmpty, ctx ValueProcessorContext) (any, error) {
			if ctx.IsChanged {
				return v, nil
			}
			switch ctx.DefParam.Type {
			case jsonschema.String:
				return "processed_default", nil
			case jsonschema.Integer:
				return 42, nil
			case jsonschema.Array:
				return []string{"1", "2", "3"}, nil
			default:
				return v, nil
			}
		},
	}
	procReplace := GenericValueProcessor[*procTestReplaceOptions]{
		Types: []jsonschema.Type{jsonschema.String},
		Fn: func(v any, opts *procTestReplaceOptions, _ ValueProcessorContext) (any, error) {
			return strings.Replace(v.(string), opts.O, opts.N, -1), nil
		},
	}
	procErr := GenericValueProcessor[*ValueProcessorOptionsEmpty]{
		Fn: func(v any, _ *ValueProcessorOptionsEmpty, ctx ValueProcessorContext) (any, error) {
			return v, fmt.Errorf("my_error %q", ctx.DefParam.Name)
		},
	}
	am.AddValueProcessor("test.defaultVal", procDefVal)
	am.AddValueProcessor("test.replace", procReplace)
	am.AddValueProcessor("test.error", procErr)
}

func Test_ActionsValueProcessor(t *testing.T) {
	am := NewManager()
	addTestValueProcessors(am)

	tt := []TestCaseValueProcessor{
		{"valid processor chain - with defaults, input given", actionProcessWithDefault, nil, nil,
			InputParams{"arg1": "AAACCC"},
			InputParams{"opt1": "ACACAC"},
			InputParams{"arg1": "BBBDDD"},
			InputParams{"opt1": "BDBDBD"},
		},
		{Name: "valid processor chain - with default, no input given", Yaml: actionProcessWithDefault, ExpArgs: InputParams{"arg1": "processed_default"}, ExpOpts: InputParams{"opt1": "processed_default"}},
		{Name: "valid processor chain - no defaults, no input given", Yaml: actionProcessNoDefault, ExpArgs: InputParams{"arg1": "processed_default"}, ExpOpts: InputParams{"opt1": "processed_default"}},
		{Name: "valid processor - array processed and cast to []any", Yaml: actionProcessArrayType, ExpArgs: InputParams{"arg1": []any{"1", "2", "3"}}, ExpOpts: InputParams{}},
		{Name: "wrong options", Yaml: actionProcessWrongOptions, ErrInit: yamlMergeErrors(yamlTypeError("line 10: cannot unmarshal !!seq into string"), yamlTypeError("line 12: cannot unmarshal !!map into string"))},
		{Name: "broken processor", Yaml: actionProcessBroken, ErrInit: fmt.Errorf(errTplNonExistProcessor, "test.broken")},
		{Name: "unsupported type", Yaml: actionProcessUnsupType, ErrInit: fmt.Errorf(errTplNotApplicableProcessor, "test.replace", jsonschema.Integer)},
		{Name: "processor return error", Yaml: actionProcessReturnErr, ErrProc: fmt.Errorf(errTplErrorOnProcessor, "arg1", "test.error", fmt.Errorf("my_error %q", "arg1"))},
	}
	for _, tt := range tt {
		tt := tt
		t.Run(tt.Name, func(t *testing.T) {
			t.Parallel()
			tt.Test(t, am)
		})
	}
}
