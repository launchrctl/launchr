package builtinprocessors

import (
	"testing"
	"testing/fstest"

	"github.com/launchrctl/launchr/internal/launchr"
	"github.com/launchrctl/launchr/pkg/action"
	"github.com/launchrctl/launchr/pkg/jsonschema"
)

const testProcGetConfig = `
runtime: plugin
action:
  title: test config
  options:
    - name: string
      process:
        - processor: config.GetValue
          options:
            path: my.string
    - name: int
      type: integer
      default: 24
      process:
        - processor: config.GetValue
          options:
            path: my.int
    - name: bool
      type: boolean
      process:
        - processor: config.GetValue
          options:
            path: my.bool
    - name: array
      type: array
      process:
        - processor: config.GetValue
          options:
            path: my.array
`

const testProcGetConfigTypeMismatch = `
runtime: plugin
action:
  title: test config
  options:
    - name: string
      process:
        - processor: config.GetValue
          options:
            path: my.int
`

const testProcGetConfigWrongDef = `
runtime: plugin
action:
  title: test config
  options:
    - name: string
      process:
        - processor: config.GetValue
`

const testConfig = `
my:
  string: my_str
  int: 42
  bool: true
  array: ["1", "2", "3"]
`

func testConfigFS(s string) launchr.Config {
	m := fstest.MapFS{
		"config.yaml": &fstest.MapFile{Data: []byte(s)},
	}
	return launchr.ConfigFromFS(m)
}

func Test_ConfigProcessor(t *testing.T) {
	// Prepare services.
	cfg := testConfigFS(testConfig)
	am := action.NewManager()
	addValueProcessors(am, cfg)

	expConfig := action.InputParams{
		"string": "my_str",
		"int":    42,
		"bool":   true,
		"array":  []any{"1", "2", "3"},
	}
	expGiven := action.InputParams{
		"string": "my_input_str",
		"int":    422,
		"bool":   false,
		"array":  []any{"3", "2", "1"},
	}
	tt := []action.TestCaseValueProcessor{
		{Name: "get config value - no input given", Yaml: testProcGetConfig, ExpOpts: expConfig},
		{Name: "get config value - input given", Yaml: testProcGetConfig, Opts: expGiven, ExpOpts: expGiven},
		{Name: "get config value - result type mismatch", Yaml: testProcGetConfigTypeMismatch, ErrProc: jsonschema.NewErrTypeMismatch(0, "")},
		{Name: "get config value - wrong options", Yaml: testProcGetConfigWrongDef, ErrInit: action.ErrValueProcessorOptionsFieldValidation{Field: "path", Reason: "required"}},
	}
	for _, tt := range tt {
		tt := tt
		t.Run(tt.Name, func(t *testing.T) {
			t.Parallel()
			tt.Test(t, am)
		})
	}
}
