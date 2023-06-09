package action

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

type testActionLoader struct {
	cfg *Config
}

func (l *testActionLoader) Content() ([]byte, error) {
	return nil, errors.New("not implemented")
}

func (l *testActionLoader) Load() (*Config, error) {
	return l.cfg, nil
}

func (l *testActionLoader) LoadRaw() (*Config, error) {
	return l.Load()
}

func testLoaderCmd() *Command {
	cfg := &Config{
		Version: "1",
		Action: &Action{
			Arguments: ArgumentsList{
				&Argument{
					Name: "arg1",
				},
			},
			Options: OptionsList{
				&Option{
					Name: "optStr",
				},
			},
		},
	}
	return &Command{
		CommandName: "my_cmd",
		Loader:      &testActionLoader{cfg: cfg},
	}
}

func Test_EnvProcessor(t *testing.T) {
	proc := &envProcessor{}
	_ = os.Setenv("TEST_ENV1", "VAL1")
	_ = os.Setenv("TEST_ENV2", "VAL2")
	s := "$TEST_ENV1,${TEST_ENV2},${TEST_ENV3-def1},${TEST_ENV_UNDEF}"
	res, _ := proc.Process([]byte(s))
	assert.Equal(t, "VAL1,VAL2,def1,", string(res))
}

func Test_InputProcessor(t *testing.T) {
	cmd := testLoaderCmd()
	proc := &inputProcessor{cmd: cmd}
	cmd.SetArgsInput([]string{"arg1"})
	cmd.SetOptsInput(map[string]interface{}{"optStr": "optVal1"})

	s := "{{ .arg1 }},{{ .optStr }}"
	res, err := proc.Process([]byte(s))
	assert.NoError(t, err)
	assert.Equal(t, "arg1,optVal1", string(res))

	s = "{{ .arg2 }},{{ .optUnd }}"
	res, err = proc.Process([]byte(s))
	assert.Error(t, err)
	assert.Equal(t, "", string(res))
}

func Test_PipeProcessor(t *testing.T) {
	cmd := testLoaderCmd()
	proc := &pipeProcessor{
		[]LoadProcessor{
			&envProcessor{},
			&inputProcessor{cmd: cmd},
		},
	}

	_ = os.Setenv("TEST_ENV1", "VAL1")
	cmd.SetArgsInput([]string{"arg1"})
	cmd.SetOptsInput(map[string]interface{}{"optStr": "optVal1"})
	s := "$TEST_ENV1,{{ .arg1 }},{{ .optStr }}"
	res, err := proc.Process([]byte(s))
	assert.NoError(t, err)
	assert.Equal(t, "VAL1,arg1,optVal1", string(res))
}
