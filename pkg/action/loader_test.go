package action

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func testLoaderAction() *Action {
	af := &Definition{
		Version: "1",
		Action: &DefAction{
			Arguments: ArgumentsList{
				&Argument{
					Name: "arg1",
				},
			},
			Options: OptionsList{
				&Option{
					Name: "optStr",
				},
				&Option{
					Name: "opt-str",
				},
			},
		},
	}
	return &Action{
		ID:     "my_actions",
		Loader: af,
	}
}

func Test_EnvProcessor(t *testing.T) {
	proc := envProcessor{}
	_ = os.Setenv("TEST_ENV1", "VAL1")
	_ = os.Setenv("TEST_ENV2", "VAL2")
	s := "$TEST_ENV1,${TEST_ENV2},${TEST_ENV3-def1},${TEST_ENV_UNDEF}"
	res, _ := proc.Process(LoadContext{}, []byte(s))
	assert.Equal(t, "VAL1,VAL2,def1,", string(res))
}

func Test_InputProcessor(t *testing.T) {
	act := testLoaderAction()
	ctx := LoadContext{Action: act}
	proc := inputProcessor{}
	err := act.SetInput(Input{TypeArgs{"arg1": "arg1"}, TypeOpts{"optStr": "optVal1", "opt-str": "opt-val2"}, nil})
	assert.NoError(t, err)

	s := "{{ .arg1 }},{{ .optStr }},{{ .opt_str }}"
	res, err := proc.Process(ctx, []byte(s))
	assert.NoError(t, err)
	assert.Equal(t, "arg1,optVal1,opt-val2", string(res))

	s = "{{ .opt-str }}"
	res, err = proc.Process(ctx, []byte(s))
	assert.ErrorContains(t, err, "Unexpected '-' symbol in a template variable.")
	assert.Equal(t, "", string(res))

	s = "{{ .arg2 }},{{ .optUnd }}"
	res, err = proc.Process(ctx, []byte(s))
	assert.Equal(t, err, errMissingVar{vars: map[string]struct{}{"optUnd": {}, "arg2": {}}})
	assert.Equal(t, "", string(res))
}

func Test_YamlTplCommentsProcessor(t *testing.T) {
	act := testLoaderAction()
	ctx := LoadContext{Action: act}
	proc := NewPipeProcessor(
		escapeYamlTplCommentsProcessor{},
		inputProcessor{},
	)
	err := act.SetInput(Input{TypeArgs{"arg1": "arg1"}, TypeOpts{"optStr": "optVal1"}, nil})
	assert.NoError(t, err)
	// Check the commented strings are not considered.
	s := `
t: "{{ .arg1 }} # {{ .optStr }}"
t: '{{ .arg1 }} # {{ .optStr }}'
t: {{ .arg1 }} # {{ .optUnd }}
# {{ .optUnd }} {{ .arg1 }}
	`
	res, err := proc.Process(ctx, []byte(s))
	assert.NoError(t, err)
	assert.Equal(t, "t: \"arg1 # optVal1\"\nt: 'arg1 # optVal1'\nt: arg1", strings.TrimSpace(string(res)))
	s = `t: "{{ .arg1 }} # {{ .optUnd }}""`
	// Check we still have an error on an undefined variable.
	res, err = proc.Process(ctx, []byte(s))
	assert.Equal(t, err, errMissingVar{vars: map[string]struct{}{"optUnd": {}}})
	assert.Equal(t, "", string(res))
}

func Test_PipeProcessor(t *testing.T) {
	act := testLoaderAction()
	ctx := LoadContext{Action: act}
	proc := NewPipeProcessor(
		envProcessor{},
		inputProcessor{},
	)

	_ = os.Setenv("TEST_ENV1", "VAL1")
	err := act.SetInput(Input{TypeArgs{"arg1": "arg1"}, TypeOpts{"optStr": "optVal1"}, nil})
	assert.NoError(t, err)
	s := "$TEST_ENV1,{{ .arg1 }},{{ .optStr }}"
	res, err := proc.Process(ctx, []byte(s))
	assert.NoError(t, err)
	assert.Equal(t, "VAL1,arg1,optVal1", string(res))
}
