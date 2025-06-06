package action

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/launchrctl/launchr/internal/launchr"
	"github.com/launchrctl/launchr/pkg/jsonschema"
)

func Test_Action(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	require := require.New(t)

	// Prepare an action.
	fsys := genFsTestMapActions(1, validFullYaml, genPathTypeValid)
	ad := NewYamlDiscovery(NewDiscoveryFS(fsys, ""))
	ctx := context.Background()
	actions, err := ad.Discover(ctx)
	require.NoError(err)
	require.NotEmpty(actions)
	act := actions[0]
	// Override the real path to skip [Action.syncToDisc].
	act.fs.real = "/fstest/"

	// Test dir
	assert.Equal(act.fs.real+filepath.Dir(act.fpath), act.Dir())
	act.fpath = "test/file/path/action.yaml"
	assert.Equal(act.fs.real+"test/file/path", act.Dir())

	// Test arguments and options.
	inputArgs := InputParams{"arg1": "arg1", "arg2": "arg2", "arg-1": "arg-1", "arg_12": "arg_12_enum1"}
	inputOpts := InputParams{
		"opt1":   "opt1val",
		"opt-1":  "opt-1",
		"opt2":   true,
		"opt3":   1,
		"opt4":   1.45,
		"optarr": []any{"opt5.1val", "opt5.2val"},
		"opt6":   "unexpectedOpt",
	}
	input := NewInput(act, inputArgs, inputOpts, nil)
	require.NotNil(input)
	input.SetValidated(true)
	err = act.SetInput(input)
	require.NoError(err)
	require.NotNil(act.input)

	// Option is not defined, but should be there
	// because [manager.ValidateInput] decides if the input correct or not.
	_, okOpt := act.input.Opts()["opt6"]
	assert.True(okOpt)
	assert.Equal(inputArgs, act.input.Args())
	assert.Equal(inputOpts, act.input.Opts())

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
	runDef := act.RuntimeDef()
	assert.Equal(execExp, []string(runDef.Container.Command))
	assert.NotNil(act.def)

	// Test image name.
	assert.Equal("my/image:v1", runDef.Container.Image)
	// Test hosts.
	extraHosts := StrSlice{
		"host.docker.internal:host-gateway",
		"example.com:127.0.0.1",
	}
	assert.Equal(extraHosts, runDef.Container.ExtraHosts)

	// Test build info
	b := act.ImageBuildInfo(runDef.Container.Image)
	assert.NotNil(b)
	tags := []string{
		"my/image:v2",
		"my/image:v3",
		"my/image:v1",
	}
	assert.Equal(tags, b.Tags)
	runDef.Container.Build = nil
	assert.Nil(nil)
}

func Test_Action_NewYAMLFromFS(t *testing.T) {
	t.Parallel()
	// Prepare FS.
	fsys := genFsTestMapActions(1, validFullYaml, genPathTypeArbitrary)
	// Get first key to make subdir.
	var key string
	for key = range fsys {
		// There is only 1 entry, we get the only key.
		break
	}

	// Create action.
	subfs, _ := fs.Sub(fsys, filepath.Dir(key))
	a, err := NewYAMLFromFS("test", subfs)
	require.NotNil(t, a)
	require.NoError(t, err)
	assert.Equal(t, "test", a.ID)
	require.NoError(t, a.EnsureLoaded())
	assert.Equal(t, "Title", a.ActionDef().Title)

	// Export from memory to disk.
	err = a.syncToDisk()
	require.NoError(t, err)

	// Check the data is properly set for accessing.
	assert.NotEmpty(t, a.fs.real)
	fpath := a.Filepath()
	assert.True(t, filepath.IsAbs(fpath))
	assert.FileExists(t, fpath)

	err = launchr.Cleanup()
	assert.NoError(t, err)
	assert.NoFileExists(t, fpath)
}

func Test_ActionInput(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	require := require.New(t)
	a := NewFromYAML("input_test", []byte(validMultipleArgsAndOpts))
	// Create empty input.
	input := NewInput(a, nil, nil, nil)
	require.NotNil(input)

	// Test validated.
	assert.False(input.IsValidated())
	input.SetValidated(true)
	assert.True(input.IsValidated())
	input.SetValidated(false)
	assert.False(input.IsValidated())

	// Test get argument that has a default value.
	arg := input.Arg("arg_default")
	assert.Equal("my_default_string", arg)
	// Get defined argument but not set.
	arg = input.Arg("arg_int")
	assert.Nil(arg)
	// Get undefined argument.
	arg = input.Arg("undefined")
	assert.Nil(arg)

	// Get defined option. Default value is not set.
	opt := input.Opt("opt_str")
	assert.Equal(nil, opt)
	// Get undefined option, value is not set.
	opt = input.Opt("undefined")
	assert.Nil(opt)

	// Test user changed input.
	// Check argument is changed.
	input = NewInput(a, InputParams{"arg_str": "my_string"}, nil, nil)
	require.NotNil(input)
	assert.Equal(InputParams{"arg_str": "my_string", "arg_default": "my_default_string"}, input.Args())
	assert.Equal(InputParams{"arg_str": "my_string"}, input.ArgsChanged())
	assert.True(input.IsArgChanged("arg_str"))
	assert.False(input.IsArgChanged("arg_int"))
	assert.False(input.IsArgChanged("arg_str2"))
	input.SetArg("arg_str2", "my_str2")
	assert.True(input.IsArgChanged("arg_str2"))
	assert.Equal(InputParams{"arg_str": "my_string", "arg_str2": "my_str2"}, input.ArgsChanged())
	input.UnsetArg("arg_str")
	assert.Equal(InputParams{"arg_str2": "my_str2"}, input.ArgsChanged())
	assert.False(input.IsArgChanged("arg_str"))
	// Check option is changed.
	input = NewInput(a, nil, InputParams{"opt_str": "my_string"}, nil)
	require.NotNil(input)
	assert.Equal(InputParams{"opt_str": "my_string"}, input.OptsChanged())
	assert.True(input.IsOptChanged("opt_str"))
	assert.False(input.IsOptChanged("opt_int"))
	// Set option and check it's changed.
	input.SetOpt("opt_int", 24)
	assert.True(input.IsOptChanged("opt_int"))
	assert.Equal(InputParams{"opt_str": "my_string", "opt_int": 24, "opt_str_default": "optdefault"}, input.Opts())
	input.UnsetOpt("opt_str")
	assert.Equal(InputParams{"opt_int": 24}, input.OptsChanged())
	assert.False(input.IsOptChanged("opt_str"))

	// Test create with positional arguments of different types.
	argsPos := []string{"42", "str", "str2", "true", "str3", "undstr", "24"}
	argsNamed, err := ArgsPosToNamed(a, argsPos)
	require.NoError(err)
	savedPos, posKeyOk := argsNamed[inputMapKeyArgsPos]
	assert.True(posKeyOk)
	assert.Equal(argsPos, savedPos)
	input = NewInput(a, argsNamed, nil, nil)
	expArgs := InputParams{
		"arg_int":     42,
		"arg_str":     "str",
		"arg_str2":    "str2",
		"arg_bool":    true,
		"arg_default": "str3",
	}
	_, posKeyOk = input.args[inputMapKeyArgsPos]
	assert.False(posKeyOk)
	assert.Equal(expArgs, input.Args())
	assert.Equal(argsPos, input.ArgsPositional())
}

func Test_ActionInputValidate(t *testing.T) {
	t.Parallel()
	type inputProcessFn func(_ *testing.T, a *Action, input *Input)
	type testCase struct {
		name   string
		yaml   string
		args   InputParams
		opts   InputParams
		fnInit inputProcessFn
		expErr error
	}

	am := NewManager()

	// Extra input preparation and testing.
	setValidatedInput := func(t *testing.T, _ *Action, input *Input) {
		input.SetValidated(true)
		assert.True(t, input.validated)
	}

	setPosArgs := func(args ...string) inputProcessFn {
		return func(t *testing.T, a *Action, input *Input) {
			argsPos, err := ArgsPosToNamed(a, args)
			require.NoError(t, err)
			*input = *NewInput(a, argsPos, input.OptsChanged(), input.Streams())
		}
	}

	// Checks that argument has expected value.
	assertArgValue := func(arg string, exp string) inputProcessFn {
		return func(t *testing.T, _ *Action, input *Input) {
			actual := input.Arg(arg)
			assert.Equal(t, exp, actual)
		}
	}

	// Argument or option property path.
	arg := func(k ...string) []string { return append([]string{jsonschemaPropArgs}, k...) }
	opt := func(k ...string) []string { return append([]string{jsonschemaPropOpts}, k...) }

	// JSON Schema errors.
	newError := func(path []string, msg string) jsonschema.ErrSchemaValidation {
		return jsonschema.NewErrSchemaValidation(path, msg)
	}

	// Creates a validation error.
	schemaErr := func(err ...jsonschema.ErrSchemaValidation) jsonschema.ErrSchemaValidationArray {
		return err
	}

	// Error of type mismatch.
	newErrExpType := func(path []string, expT string, actT string) jsonschema.ErrSchemaValidation {
		return newError(path, fmt.Sprintf("got %s, want %s", actT, expT))
	}

	joinQuoted := func(s []string, sep string) string {
		quoted := make([]string, len(s))
		for i := 0; i < len(s); i++ {
			quoted[i] = `'` + s[i] + `'`
		}
		return strings.Join(quoted, sep)
	}

	// Error when property is missing.
	newErrMissProp := func(path []string, props ...string) jsonschema.ErrSchemaValidation {
		if len(props) == 1 {
			return newError(path, fmt.Sprintf("missing property %s", joinQuoted(props, ", ")))
		}
		return newError(path, fmt.Sprintf("missing properties %s", joinQuoted(props, ", ")))
	}

	newErrAddProps := func(path []string, props ...string) jsonschema.ErrSchemaValidation {
		return newError(path, fmt.Sprintf("additional properties %s not allowed", joinQuoted(props, ", ")))
	}

	// Error of enum.
	newErrEnum := func(path []string, enums ...string) jsonschema.ErrSchemaValidation {
		return newError(path, fmt.Sprintf(`value must be one of %s`, joinQuoted(enums, ", ")))
	}

	tt := []testCase{
		{"valid arg string", validArgString, InputParams{"arg_string": "arg1"}, nil, nil, nil},
		{"valid arg string - undefined arg and opt", validArgString, InputParams{"arg_string": "arg1", "arg_undefined": "und"}, InputParams{"opt_undefined": "und"}, nil, schemaErr(
			newErrAddProps(arg(), "arg_undefined"),
			newErrAddProps(opt(), "opt_undefined"),
		)},
		{"valid args positional", validArgString, nil, nil, setPosArgs("arg1"), nil},
		{"invalid args positional - given more than expected", validArgString, nil, nil, setPosArgs("arg1", "arg2"),
			fmt.Errorf("accepts 1 arg(s), received 2"),
		},
		{"invalid arg string - number given", validArgString, InputParams{"arg_string": 1}, nil, nil, schemaErr(
			newErrExpType(arg("arg_string"), "string", "number"),
		)},
		{"invalid required - arg not given", validArgString, InputParams{}, nil, nil, schemaErr(
			newErrMissProp(arg(), "arg_string"),
		)},
		{"invalid required ok - validation skipped", validArgString, InputParams{}, nil, setValidatedInput, nil},
		{"valid arg optional", validArgStringOptional, InputParams{}, nil, nil, nil},
		{"valid arg string enum", validArgStringEnum, InputParams{"arg_enum": "enum1"}, nil, nil, nil},
		{"invalid arg string enum - number given", validArgStringEnum, InputParams{"arg_enum": 1}, nil, nil, schemaErr(
			newErrExpType(arg("arg_enum"), "string", "number"),
		)},
		{"invalid arg string enum - incorrect enum given", validArgStringEnum, InputParams{"arg_enum": "invalid"}, nil, nil, schemaErr(
			newErrEnum(arg("arg_enum"), "enum1", "enum2"),
		)},
		{"valid arg boolean", validArgBoolean, InputParams{"arg_boolean": true}, nil, nil, nil},
		{"valid arg default - correct type given", validArgDefault, InputParams{"arg_default": "my_val"}, nil, assertArgValue("arg_default", "my_val"), nil},
		{"invalid arg default - wrong type given", validArgDefault, InputParams{"arg_default": true}, nil, nil, schemaErr(
			newErrExpType(arg("arg_default"), "string", "boolean"),
		)},
		{"valid arg default - arg not given", validArgDefault, InputParams{}, nil, assertArgValue("arg_default", "default_string"), nil},
		{"valid boolean opt", validOptBoolean, nil, InputParams{"opt_boolean": true}, nil, nil},
		{"invalid boolean opt - string given", validOptBoolean, nil, InputParams{"opt_boolean": "str"}, nil, schemaErr(
			newErrExpType(opt("opt_boolean"), "boolean", "string"),
		)},
		{"valid array type string - string slice given", validOptArrayImplicitString, nil, InputParams{"opt_array_str": []string{"str1", "str2"}}, nil, nil},
		{"valid array type string - any slice given", validOptArrayImplicitString, nil, InputParams{"opt_array_str": []any{"str1", "str2"}}, nil, nil},
		{"invalid array type string - int slice given", validOptArrayImplicitString, nil, InputParams{"opt_array_str": []int{1, 2, 3}}, nil, schemaErr(
			newErrExpType(opt("opt_array_str", "0"), "string", "number"),
			newErrExpType(opt("opt_array_str", "1"), "string", "number"),
			newErrExpType(opt("opt_array_str", "2"), "string", "number"),
		)},
		{"valid array type string enum", validOptArrayStringEnum, nil, InputParams{"opt_array_enum": []string{"enum_arr1", "enum_arr2"}}, nil, nil},
		{"invalid array type string enum - incorrect enum given", validOptArrayStringEnum, nil, InputParams{"opt_array_enum": []string{"enum_arr_incorrect1", "enum_arr_incorrect2"}}, nil, schemaErr(
			newErrEnum(opt("opt_array_enum", "0"), "enum_arr1", "enum_arr2"),
			newErrEnum(opt("opt_array_enum", "1"), "enum_arr1", "enum_arr2"),
		)},
		{"valid array type integer", validOptArrayInt, nil, InputParams{"opt_array_int": []int{1, 2, 3}}, nil, nil},
		{"valid array type integer - default used", validOptArrayIntDefault, nil, nil, nil, nil},
		{"valid multiple args and opts", validMultipleArgsAndOpts, InputParams{"arg_int": 1, "arg_str": "mystr", "arg_str2": "mystr", "arg_bool": true}, InputParams{"opt_str_required": "mystr"}, nil, nil},
		{"invalid multiple args and opts - multiple causes", validMultipleArgsAndOpts, InputParams{"arg_int": "str", "arg_str": 1}, InputParams{"opt_str": 1}, nil, schemaErr(
			newErrMissProp(arg(), "arg_str2", "arg_bool"),
			newErrExpType(arg("arg_int"), "integer", "string"),
			newErrExpType(arg("arg_str"), "string", "number"),
			newErrMissProp(opt(), "opt_str_required"),
			newErrExpType(opt("opt_str"), "string", "number"),
		)},
		{"valid format and pattern - email and uppercase given", validPatternFormat, InputParams{"arg_email": "my@example.com", "arg_pattern": "UPPER"}, nil, nil, nil},
		{"invalid format and pattern - wrong email and lowercase given", validPatternFormat, InputParams{"arg_email": "not_email", "arg_pattern": "lower"}, nil, nil, schemaErr(
			newError(arg("arg_email"), "'not_email' is not valid email: missing @"),
			newError(arg("arg_pattern"), "'lower' does not match pattern '^[A-Z]+$'"),
		)},
	}

	for _, tt := range tt {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a := NewFromYAML(tt.name, []byte(tt.yaml))
			input := NewInput(a, tt.args, tt.opts, nil)
			require.NotNil(t, input)
			if tt.fnInit != nil {
				tt.fnInit(t, a, input)
			}
			err := am.ValidateInput(a, input)
			assert.Equal(t, err == nil, input.IsValidated())
			assertIsSameError(t, tt.expErr, err)
		})
	}
}
