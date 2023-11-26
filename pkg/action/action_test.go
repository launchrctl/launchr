package action

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Action(t *testing.T) {
	assert := assert.New(t)
	// Prepare an action.
	fs := _getFsMapActions(1, validFullYaml, true)
	ad := NewYamlDiscovery(fs)
	actions, err := ad.Discover()
	assert.NoError(err)
	assert.NotEmpty(actions)
	act := actions[0]
	err = act.EnsureLoaded()
	assert.NoError(err)
	actConf := act.ActionDef()
	// Test image name.
	assert.Equal("my/image:v1", actConf.Image)
	// Test dir
	assert.Equal(filepath.Dir(act.fpath), act.Dir())
	act.fpath = "test/file/path/action.yaml"
	assert.Equal("test/file/path", act.Dir())
	// Test hosts.
	extraHosts := StrSlice{
		"host.docker.internal:host-gateway",
		"example.com:127.0.0.1",
	}
	assert.Equal(extraHosts, actConf.ExtraHosts)
	// Test arguments and options.
	inputArgs := TypeArgs{"arg1": "arg1", "arg2": "arg2", "arg-1": "arg-1", "arg_12": "arg_12"}
	inputOpts := TypeOpts{
		"opt1":   "opt1val",
		"opt-1":  "opt-1",
		"opt2":   true,
		"opt3":   1,
		"opt4":   1.45,
		"optarr": []interface{}{"opt5.1val", "opt5.2val"},
		"opt6":   "unexpectedOpt",
	}
	err = act.SetInput(Input{inputArgs, inputOpts, nil})
	assert.NoError(err)
	assert.Equal(inputArgs, act.input.Args)
	assert.Equal(inputOpts, act.input.Opts)

	// Test templating in executable.
	envVar1 := "envval1"
	_ = os.Setenv("TEST_ENV_1", envVar1)
	execExp := []string{
		"/bin/sh",
		"-c",
		"ls -lah",
		fmt.Sprintf("%v %v %v %v", inputArgs["arg2"], inputArgs["arg1"], inputArgs["arg-1"], inputArgs["arg_12"]),
		fmt.Sprintf("%v %v %v %v %v %v", inputOpts["opt3"], inputOpts["opt2"], inputOpts["opt1"], inputOpts["opt-1"], inputOpts["opt4"], inputOpts["optarr"]),
		fmt.Sprintf("%v", envVar1),
		fmt.Sprintf("%v ", envVar1),
	}
	act.Reset()
	err = act.EnsureLoaded()
	assert.NoError(err)
	actConf = act.ActionDef()
	assert.Equal(execExp, []string(actConf.Command))
	assert.NotNil(act.def)

	// Test build info
	b := act.ImageBuildInfo(actConf.Image)
	assert.NotNil(b)
	tags := []string{
		"my/image:v2",
		"my/image:v3",
		"my/image:v1",
	}
	assert.Equal(tags, b.Tags)
	actConf.Build = nil
	assert.Nil(nil)
}
