package builtinprocessors

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

const testTplConfig = `
action:
  title: test config
runtime:
  type: container
  image: alpine
  command:
    - '{{ config "my.string" }}'
    - '{{ config "my.int" }}'
    - '{{ config "my.bool" }}'
    - '{{ config "my.array" }}'
    - '{{ index (config "my.array") 1 }}'
    - '{{ config "my.null" | default "foo" }}'
    - '{{ config "my.missing" | default "bar" }}'
`

const testTplConfigMissing = `
action:
  title: test config
runtime:
  type: container
  image: alpine
  command:
    - '{{ config "my.missing" }}'
`

const testTplConfigBadArgs = `
action:
  title: test config
runtime:
  type: container
  image: alpine
  command:
    - '{{ config "my.string" "my.string" }}'
`

const testConfig = `
my:
  string: my_str
  int: 42
  bool: true
  array: ["1", "2", "3"]
  null: null
`

const testTplYq = `
action:
  title: test YamlQuery
  options:
    - name: yamlPath
      default: "foo/bar.yaml"
runtime:
  type: container
  image: alpine
  command:
    - '{{ YamlQuery .yamlPath "foo.bar" }}'
    - '{{ index (YamlQuery .yamlPath "foo") "bar" }}'
    - '{{ YamlQuery .yamlPath "foo.null" | default "foo" }}'
    - '{{ YamlQuery .yamlPath "my.missing" | default "bar" }}'
`

const testTplYqMissing = `
action:
  title: test YamlQuery
  options:
    - name: yamlPath
      default: "foo/bar.yaml"
runtime:
  type: container
  image: alpine
  command:
    - '{{ YamlQuery .yamlPath "my.missing" }}'
`

const testTplYqBadArgs = `
action:
  title: test YamlQuery
  options:
    - name: yamlPath
      default: "foo/bar.yaml"
runtime:
  type: container
  image: alpine
  command:
    - '{{ YamlQuery "1" "2" "3" }}'
`

const testYqFileContent = `
foo:
  bar: buz
  null: null
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
	tp := action.NewTemplateProcessors()
	addValueProcessors(tp, cfg)

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
			tt.Test(t, am, tp)
		})
	}
}

func Test_ConfigTemplateFunc(t *testing.T) {
	// Prepare services.
	cfg := testConfigFS(testConfig)
	tp := action.NewTemplateProcessors()
	addValueProcessors(tp, cfg)
	svc := launchr.NewServiceManager()
	svc.Add(tp)

	type testCase struct {
		Name string
		Yaml string
		Exp  []string
		Err  string
	}

	tt := []testCase{
		{Name: "valid", Yaml: testTplConfig, Exp: []string{"my_str", "42", "true", "[1 2 3]", "2", "foo", "bar"}},
		{Name: "key not found", Yaml: testTplConfigMissing, Exp: []string{"<key not found \"my.missing\">"}},
		{Name: "incorrect call", Yaml: testTplConfigBadArgs, Err: "wrong number of args for config: want 1 got 2"},
	}
	for _, tt := range tt {
		tt := tt
		t.Run(tt.Name, func(t *testing.T) {
			t.Parallel()
			a := action.NewFromYAML(tt.Name, []byte(tt.Yaml))
			a.SetServices(svc)
			err := a.EnsureLoaded()
			if tt.Err != "" {
				require.ErrorContains(t, err, tt.Err)
				return
			}
			require.NoError(t, err)
			rdef := a.RuntimeDef()
			assert.Equal(t, tt.Exp, []string(rdef.Container.Command))
		})
	}
}

func Test_YqTemplateFunc(t *testing.T) {
	// Prepare services.
	tp := action.NewTemplateProcessors()
	addValueProcessors(tp, nil)
	svc := launchr.NewServiceManager()
	svc.Add(tp)

	// Prepare test data.
	wd := t.TempDir()
	err := os.MkdirAll(filepath.Join(wd, "foo"), 0750)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(wd, "foo", "bar.yaml"), []byte(testYqFileContent), 0600)
	require.NoError(t, err)

	type testCase struct {
		Name string
		Yaml string
		Exp  []string
		Err  string
	}

	tt := []testCase{
		{Name: "valid", Yaml: testTplYq, Exp: []string{"buz", "buz", "foo", "bar"}},
		{Name: "key not found", Yaml: testTplYqMissing, Exp: []string{"<key not found \"foo/bar.yaml:my.missing\">"}},
		{Name: "incorrect call", Yaml: testTplYqBadArgs, Err: "wrong number of args for YamlQuery: want 2 got 3"},
	}
	for _, tt := range tt {
		tt := tt
		t.Run(tt.Name, func(t *testing.T) {
			t.Parallel()
			a := action.NewFromYAML(tt.Name, []byte(tt.Yaml))
			a.SetWorkDir(wd)
			a.SetServices(svc)
			err := a.EnsureLoaded()
			if tt.Err != "" {
				require.ErrorContains(t, err, tt.Err)
				return
			}
			require.NoError(t, err)
			rdef := a.RuntimeDef()
			assert.Equal(t, tt.Exp, []string(rdef.Container.Command))
		})
	}
}
