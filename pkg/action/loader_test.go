package action

import (
	"fmt"
	"os"
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
		_ = os.Unsetenv("TEST_ENVPROC1")
		_ = os.Unsetenv("TEST_ENVPROC2")
	}()
	_ = os.Setenv("TEST_ENVPROC1", "VAL1")
	_ = os.Setenv("TEST_ENVPROC2", "VAL2")
	s := "$TEST_ENVPROC1$TEST_ENVPROC1,${TEST_ENVPROC2},$$TEST_ENVPROC1,${TEST_ENVPROC_UNDEF},${TEST_ENVPROC_UNDEF-$TEST_ENVPROC1},${TEST_ENVPROC_UNDEF:-$TEST_ENVPROC2},${TEST_ENVPROC2+$TEST_ENVPROC1},${TEST_ENVPROC1:+$TEST_ENVPROC2}"
	res, err := proc.Process(&LoadContext{Action: act}, s)
	assert.NoError(t, err)
	assert.Equal(t, "VAL1VAL1,VAL2,$TEST_ENVPROC1,,VAL1,VAL2,VAL1,VAL2", string(res))
	// Test action predefined env variables.
	s = "$CBIN,$ACTION_ID,$ACTION_WD,$ACTION_DIR,$DISCOVERY_DIR"
	res, err = proc.Process(&LoadContext{Action: act}, s)
	exp := fmt.Sprintf("%s,%s,%s,%s,%s", launchr.Executable(), act.ID, act.WorkDir(), act.Dir(), act.fs.Realpath())
	assert.NoError(t, err)
	assert.Equal(t, exp, string(res))
}

func Test_InputProcessor(t *testing.T) {
	t.Parallel()
	act := testLoaderAction()
	ctx := &LoadContext{Action: act}
	proc := inputProcessor{}
	input := NewInput(act, InputParams{"arg1": "arg1"}, InputParams{"optStr": "optVal1", "opt-str": "opt-val2"}, nil)
	input.SetValidated(true)
	err := act.SetInput(input)
	require.NoError(t, err)

	// Check all available variables are replaced.
	s := "{{ .arg1 }},{{ .optStr }},{{ .opt_str }}"
	res, err := proc.Process(ctx, s)
	assert.NoError(t, err)
	assert.Equal(t, "arg1,optVal1,opt-val2", res)

	// Check the variable has incorrect name and correct error is returned.
	s = "{{ .opt-str }}"
	res, err = proc.Process(ctx, s)
	assert.ErrorContains(t, err, "unexpected '-' symbol in a template variable.")
	assert.Equal(t, s, res)

	// Check that we have an error when missing variables are not handled.
	errMissVars := errMissingVar{vars: map[string]struct{}{"optUnd": {}, "arg2": {}}}
	s = "{{ .arg2 }},{{ .optUnd }}"
	res, err = proc.Process(ctx, s)
	assert.Equal(t, errMissVars, err)
	assert.Equal(t, s, res)

	// Remove line if a variable not exists or is nil.
	s = `- "{{ if not (isNil .arg1) }}arg1 is not nil{{end}}"
- "{{ if (isNil .optUnd) }}optUnd{{ end }}"`
	res, err = proc.Process(ctx, s)
	assert.NoError(t, err)
	assert.Equal(t, "- \"arg1 is not nil\"\n- \"optUnd\"", res)

	// Remove line if a variable not exists or is nil, 1 argument is not defined and not checked.
	s = `- "{{ .arg2 }}"
- "{{ if not (isNil .arg1) }}arg1 is not nil{{end}}"
- "{{ if (isNil .optUnd) }}optUnd{{ end }}"`
	_, err = proc.Process(ctx, s)
	assert.Equal(t, errMissVars, err)

	s = `- "{{ if isSet .arg1 }}arg1 is set{{end}}"
- "{{ if isChanged .arg1 }}arg1 is changed{{end}}"`
	res, err = proc.Process(ctx, s)
	assert.NoError(t, err)
	assert.Equal(t, "- \"arg1 is set\"\n- \"arg1 is changed\"", res)
}

func Test_PipeProcessor(t *testing.T) {
	t.Parallel()
	act := testLoaderAction()
	ctx := &LoadContext{Action: act}
	proc := NewPipeProcessor(
		envProcessor{},
		inputProcessor{},
	)

	_ = os.Setenv("TEST_ENVPROC3", "VAL1")
	defer func() {
		_ = os.Unsetenv("TEST_ENVPROC3")
	}()
	input := NewInput(act, InputParams{"arg1": "arg1"}, InputParams{"optStr": "optVal1"}, nil)
	input.SetValidated(true)
	err := act.SetInput(input)
	require.NoError(t, err)
	s := "$TEST_ENVPROC3,{{ .arg1 }},{{ .optStr }}"
	res, err := proc.Process(ctx, s)
	require.NoError(t, err)
	assert.Equal(t, "VAL1,arg1,optVal1", res)
}
