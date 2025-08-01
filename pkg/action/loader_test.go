package action

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/launchrctl/launchr/internal/launchr"
)

func testLoaderAction() *Action {
	af := &Definition{
		Version: "1",
		Action: &DefAction{
			Arguments: ParametersList{
				&DefParameter{
					Name: "arg1",
				},
			},
			Options: ParametersList{
				&DefParameter{
					Name: "optStr",
				},
				&DefParameter{
					Name: "opt-str",
				},
			},
		},
	}
	a := New(StringID("my_actions"), af, NewDiscoveryFS(nil, ""), "")
	return a
}

func Test_EnvProcessor(t *testing.T) {
	t.Parallel()
	act := testLoaderAction()
	proc := envProcessor{}
	defer func() {
		_ = os.Unsetenv("TEST_ENV1")
		_ = os.Unsetenv("TEST_ENV2")
	}()
	_ = os.Setenv("TEST_ENV1", "VAL1")
	_ = os.Setenv("TEST_ENV2", "VAL2")
	s := "$TEST_ENV1$TEST_ENV1,${TEST_ENV2},$$TEST_ENV1,${TEST_ENV_UNDEF},${TODO-$TEST_ENV1},${TODO:-$TEST_ENV1},${TODO+$TEST_ENV1},${TODO:+$TEST_ENV1}"
	res, err := proc.Process(LoadContext{Action: act}, []byte(s))
	assert.NoError(t, err)
	assert.Equal(t, "VAL1VAL1,VAL2,$TEST_ENV1,,,,,", string(res))
	// Test action predefined env variables.
	s = "$CBIN,$ACTION_ID,$ACTION_WD,$ACTION_DIR,$DISCOVERY_DIR"
	res, err = proc.Process(LoadContext{Action: act}, []byte(s))
	exp := fmt.Sprintf("%s,%s,%s,%s,%s", launchr.Executable(), act.ID, act.WorkDir(), act.Dir(), act.fs.Realpath())
	assert.NoError(t, err)
	assert.Equal(t, exp, string(res))
}

func Test_InputProcessor(t *testing.T) {
	t.Parallel()
	act := testLoaderAction()
	ctx := LoadContext{Action: act}
	proc := inputProcessor{}
	input := NewInput(act, InputParams{"arg1": "arg1"}, InputParams{"optStr": "optVal1", "opt-str": "opt-val2"}, nil)
	input.SetValidated(true)
	err := act.SetInput(input)
	require.NoError(t, err)

	// Check all available variables are replaced.
	s := "{{ .arg1 }},{{ .optStr }},{{ .opt_str }}"
	res, err := proc.Process(ctx, []byte(s))
	assert.NoError(t, err)
	assert.Equal(t, "arg1,optVal1,opt-val2", string(res))

	// Check the variable has incorrect name and correct error is returned.
	s = "{{ .opt-str }}"
	res, err = proc.Process(ctx, []byte(s))
	assert.ErrorContains(t, err, "unexpected '-' symbol in a template variable.")
	assert.Equal(t, "", string(res))

	// Check that we have an error when missing variables are not handled.
	errMissVars := errMissingVar{vars: map[string]struct{}{"optUnd": {}, "arg2": {}}}
	s = "{{ .arg2 }},{{ .optUnd }}"
	res, err = proc.Process(ctx, []byte(s))
	assert.Equal(t, errMissVars, err)
	assert.Equal(t, "", string(res))

	// Remove line if a variable not exists or is nil.
	s = `- "{{ .arg1 | removeLineIfNil }}"
- "{{ .optUnd | removeLineIfNil }}" # Piping with new line
- "{{ if not (isNil .arg1) }}arg1 is not nil{{end}}"
- "{{ if (isNil .optUnd) }}{{ removeLine }}{{ end }}" # Function call without new line`
	res, err = proc.Process(ctx, []byte(s))
	assert.NoError(t, err)
	assert.Equal(t, "- \"arg1\"\n- \"arg1 is not nil\"\n", string(res))

	// Remove line if a variable not exists or is nil, 1 argument is not defined and not checked.
	s = `- "{{ .arg1 | removeLineIfNil }}"
- "{{ .optUnd|removeLineIfNil }}" # Piping with new line
- "{{ .arg2 }}"
- "{{ if not (isNil .arg1) }}arg1 is not nil{{end}}"
- "{{ if (isNil .optUnd) }}{{ removeLine }}{{ end }}" # Function call without new line`
	_, err = proc.Process(ctx, []byte(s))
	assert.Equal(t, errMissVars, err)

	s = `- "{{ if isSet .arg1 }}arg1 is set"{{end}}
- "{{ removeLineIfSet .arg1 }}" # Function call without new line
- "{{ if isChanged .arg1 }}arg1 is changed{{end}}"
- '{{ removeLineIfNotChanged "arg1" }}'
- '{{ removeLineIfChanged "arg1" }}' # Function call without new line`
	res, err = proc.Process(ctx, []byte(s))
	assert.NoError(t, err)
	assert.Equal(t, "- \"arg1 is set\"\n- \"arg1 is changed\"\n- 'arg1'\n", string(res))
}

func Test_YamlTplCommentsProcessor(t *testing.T) {
	t.Parallel()
	act := testLoaderAction()
	ctx := LoadContext{Action: act}
	proc := NewPipeProcessor(
		escapeYamlTplCommentsProcessor{},
		inputProcessor{},
	)
	input := NewInput(act, InputParams{"arg1": "arg1"}, InputParams{"optStr": "optVal1"}, nil)
	input.SetValidated(true)
	err := act.SetInput(input)
	require.NoError(t, err)
	// Check the commented strings are not considered.
	s := `
t: "{{ .arg1 }} # {{ .optStr }}"
t: '{{ .arg1 }} # {{ .optStr }}'
t: {{ .arg1 }} # {{ .optUnd }}
# {{ .optUnd }} {{ .arg1 }}
	`
	res, err := proc.Process(ctx, []byte(s))
	require.NoError(t, err)
	assert.Equal(t, "t: \"arg1 # optVal1\"\nt: 'arg1 # optVal1'\nt: arg1", strings.TrimSpace(string(res)))
	s = `t: "{{ .arg1 }} # {{ .optUnd }}""`
	// Check we still have an error on an undefined variable.
	res, err = proc.Process(ctx, []byte(s))
	assert.Equal(t, err, errMissingVar{vars: map[string]struct{}{"optUnd": {}}})
	assert.Equal(t, "", string(res))
}

func Test_PipeProcessor(t *testing.T) {
	t.Parallel()
	act := testLoaderAction()
	ctx := LoadContext{Action: act}
	proc := NewPipeProcessor(
		envProcessor{},
		inputProcessor{},
	)

	_ = os.Setenv("TEST_ENV1", "VAL1")
	defer func() {
		_ = os.Unsetenv("TEST_ENV1")
	}()
	input := NewInput(act, InputParams{"arg1": "arg1"}, InputParams{"optStr": "optVal1"}, nil)
	input.SetValidated(true)
	err := act.SetInput(input)
	require.NoError(t, err)
	s := "$TEST_ENV1,{{ .arg1 }},{{ .optStr }}"
	res, err := proc.Process(ctx, []byte(s))
	require.NoError(t, err)
	assert.Equal(t, "VAL1,arg1,optVal1", string(res))
}
