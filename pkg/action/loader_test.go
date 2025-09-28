package action

import (
	"bytes"
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
	res, err := proc.Process(&LoadContext{a: act}, s)
	assert.NoError(t, err)
	assert.Equal(t, "VAL1VAL1,VAL2,$TEST_ENVPROC1,,VAL1,VAL2,VAL1,VAL2", res)
	// Test action predefined env variables.
	s = "$CBIN,$ACTION_ID,$ACTION_WD,$ACTION_DIR,$DISCOVERY_DIR"
	res, err = proc.Process(&LoadContext{a: act}, s)
	exp := fmt.Sprintf("%s,%s,%s,%s,%s", launchr.Executable(), act.ID, act.WorkDir(), act.Dir(), act.fs.Realpath())
	assert.NoError(t, err)
	assert.Equal(t, exp, res)
}

func Test_InputProcessor(t *testing.T) {
	t.Parallel()
	act := testLoaderAction()
	ctx := &LoadContext{a: act, svc: launchr.NewServiceManager()}
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

	// Test if a variable not exists or is nil.
	s = `- "{{ if not (isNil .arg1) }}arg1 is not nil{{end}}"
- "{{ if (isNil .optUnd) }}optUnd{{ end }}"`
	res, err = proc.Process(ctx, s)
	assert.NoError(t, err)
	assert.Equal(t, "- \"arg1 is not nil\"\n- \"optUnd\"", res)

	// Test isSet and isChanged.
	s = `- "{{ if isSet .arg1 }}arg1 is set{{end}}"
- "{{ if isChanged .arg1 }}arg1 is changed{{end}}"`
	res, err = proc.Process(ctx, s)
	assert.NoError(t, err)
	assert.Equal(t, "- \"arg1 is set\"\n- \"arg1 is changed\"", res)

	// Prepare a new load context for masked output.
	ctx = &LoadContext{a: act, svc: launchr.NewServiceManager()}
	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	streams := launchr.NewBasicStreams(nil, outBuf, errBuf)
	mySecret := "my_secret_input"
	input = NewInput(act, InputParams{"arg1": mySecret}, nil, streams)
	input.SetValidated(true)
	err = act.SetInput(input)
	assert.NoError(t, err)

	// Test we can mask sensitive data in a template when used in output.
	s = `{{ .arg1 | mask }}`
	res, err = proc.Process(ctx, s)
	assert.NoError(t, err)
	assert.Equal(t, mySecret, res)
	// Test output was masked.
	_, _ = act.Input().Streams().Out().Write([]byte(mySecret))
	_, _ = act.Input().Streams().Err().Write([]byte(mySecret))
	assert.Equal(t, "****", outBuf.String())
	assert.Equal(t, "****", errBuf.String())
	// Clean buffer for clean comparison
	outBuf.Reset()
	errBuf.Reset()
	// Test original streams were not affected.
	_, _ = streams.Out().Write([]byte(mySecret))
	_, _ = streams.Err().Write([]byte(mySecret))
	assert.Equal(t, mySecret, outBuf.String())
	assert.Equal(t, mySecret, errBuf.String())
	outBuf.Reset()
	errBuf.Reset()

	// Test previous run doesn't affect new runs.
	s = `{{ .arg1 }}`
	input = NewInput(act, InputParams{"arg1": mySecret}, nil, streams)
	input.SetValidated(true)
	err = act.SetInput(input)
	assert.NoError(t, err)
	res, err = proc.Process(ctx, s)
	assert.NoError(t, err)
	assert.Equal(t, mySecret, res)
	_, _ = act.Input().Streams().Out().Write([]byte(mySecret))
	_, _ = act.Input().Streams().Err().Write([]byte(mySecret))
	assert.Equal(t, mySecret, outBuf.String())
	assert.Equal(t, mySecret, errBuf.String())
}

func Test_PipeProcessor(t *testing.T) {
	t.Parallel()
	act := testLoaderAction()
	ctx := &LoadContext{a: act, svc: launchr.NewServiceManager()}
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
