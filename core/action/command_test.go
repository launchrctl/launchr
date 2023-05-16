package action

import (
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Command(t *testing.T) {
	assert := assert.New(t)
	// Prepare command.
	fs := _getFsMapActions(1, validFullYaml, true)
	ad := NewYamlDiscovery(fs)
	cmds, err := ad.Discover()
	assert.NoError(err)
	assert.NotEmpty(cmds)
	cmd := cmds[0]
	err = cmd.Compile()
	assert.NoError(err)
	a := cmd.Action()
	// Test image name.
	assert.Equal("my/image:v1", a.Image)
	// Test dir
	assert.Equal(path.Dir(cmd.Filepath), cmd.Dir())
	cmd.Filepath = "test/file/path/action.yaml"
	assert.Equal("test/file/path", cmd.Dir())
	// Test hosts.
	extraHosts := []string{
		"host.docker.internal:host-gateway",
		"example.com:127.0.0.1",
	}
	assert.Equal(extraHosts, a.ExtraHosts)
	// Test arguments.
	envVar1 := "envval1"
	_ = os.Setenv("TEST_ENV_1", envVar1)
	inputArgs := []string{"arg1", "arg2", "arg3"}
	cmd.SetArgsInput(inputArgs)
	assert.Equal(map[string]string{"arg1": "arg1", "arg2": "arg2"}, cmd.InputArgs)
	// Test options.
	inputOpts := map[string]interface{}{
		"opt1":   "opt1val",
		"opt2":   true,
		"opt3":   1,
		"opt4":   1.45,
		"optarr": []string{"opt5.1val", "opt5.2val"},
		"opt6":   "unexpectedOpt",
	}
	cmd.SetOptsInput(inputOpts)
	assert.Equal(inputOpts, cmd.InputOptions)

	// Test validation.
	assert.NoError(cmd.ValidateInput())

	// Test templating in executable.
	execExp := []string{
		"/bin/sh",
		"-c",
		"ls -lah",
		fmt.Sprintf("%v %v", inputArgs[1], inputArgs[0]),
		fmt.Sprintf("%v %v %v %v %v", inputOpts["opt3"], inputOpts["opt2"], inputOpts["opt1"], inputOpts["opt4"], inputOpts["optarr"]),
		fmt.Sprintf("%v", envVar1),
		fmt.Sprintf("%v ", envVar1),
	}
	cmd.cfg = nil
	err = cmd.Compile()
	assert.NoError(err)
	a = cmd.Action()
	assert.Equal(execExp, []string(a.Command))
	assert.NotNil(cmd.cfg)

	// Test build info
	b := a.BuildDefinition(cmd.Dir())
	assert.NotNil(b)
	tags := []string{
		"my/image:v2",
		"my/image:v3",
		"my/image:v1",
	}
	assert.Equal(tags, b.Tags)
	a.Build = nil
	assert.Nil(nil)
}
