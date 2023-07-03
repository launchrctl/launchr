package action

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_CreateFromYaml(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name   string
		input  string
		expErr error
	}

	errAny := errors.New("any")

	ttYaml := []testCase{
		// Yaml file is valid v1.
		{"valid yaml v1", validFullYaml, nil},
		// Valid, version is set implicitly to v1.
		{"valid empty version yaml v1", validEmptyVersionYaml, nil},
		// Version >v1 is unsupported.
		{"unsupported version >=1", unsupportedVersionYaml, errUnsupportedActionVersion{"2"}},

		// Image field in not provided v1.
		{"empty image field v1", invalidEmptyImgYaml, errEmptyActionImg},
		// Command field in not provided v1.
		{"empty command field v1", invalidEmptyCmdYaml, errEmptyActionCmd},

		// @todo provide example for each case with correct arguments/options.
		// Arguments are incorrectly provided v1 - string, not an array of objects.
		{"invalid arguments field - string v1", invalidArgsStringYaml, errFieldMustBeArr},
		// Arguments are incorrectly provided v1 - array of strings, not an array of objects.
		{"invalid arguments field - strings array", invalidArgsStringArrYaml, errArrElMustBeObj},
		// Arguments are incorrectly provided v1 - object, not an array of objects.
		{"invalid arguments field - object", invalidArgsObjYaml, errFieldMustBeArr},
		{"invalid argument empty name", invalidArgsEmptyNameYaml, errEmptyActionArgName},
		{"invalid argument name", invalidArgsNameYaml, errInvalidActionArgName},

		// Options are incorrectly provided v1 - string, not an array of objects.
		{"invalid options field - string", invalidOptsStrYaml, errFieldMustBeArr},
		// Options are incorrectly provided v1 - array of strings, not an array of objects.
		{"invalid options field - string array", invalidOptsStrArrYaml, errArrElMustBeObj},
		// Options are incorrectly provided v1 - object, not an array of objects.
		{"invalid options field - object", invalidOptsObjYaml, errFieldMustBeArr},
		{"invalid option empty name", invalidOptsEmptyNameYaml, errEmptyActionOptName},
		{"invalid option name", invalidOptsNameYaml, errInvalidActionOptName},
		{"invalid duplicate argument/option name", invalidDupArgsOptsNameYaml, errDuplicateActionName},

		// Command declaration as array of strings.
		{"valid command - strings array", validCmdArrYaml, nil},
		{"invalid command - object", invalidCmdObjYaml, errArrOrStrEl},
		{"invalid command - various array", invalidCmdArrVarYaml, errArrOrStrEl},

		// Build image.
		{"build image - short", validBuildImgShortYaml, nil},
		{"build image - long", validBuildImgLongYaml, nil},

		// Extra hosts.
		{"extra hosts", validExtraHostsYaml, nil},

		// Env variables replacement.
		{"env variables string array", validEnvArr, nil},
		{"env variables map", validEnvObj, nil},
		{"invalid env variables", invalidEnv, errAny},

		// Templating.
		{"unescaped template val", validUnescTplStr, errAny},
	}
	for _, tt := range ttYaml {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := CreateFromYaml(bytes.NewReader([]byte(tt.input)))
			if tt.expErr == errAny {
				assert.Error(t, err)
			} else if !errors.Is(err, tt.expErr) {
				t.Errorf("expected error %v, got %v", tt.expErr, err)
			}
		})
	}

	// @todo test that the content is in place
}

func Test_CreateFromYamlTpl(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name   string
		input  string
		expErr error
	}
	errAny := errors.New("any")

	ttYaml := []testCase{
		// Templating. @todo
		{"supported unescaped template val", validUnescTplStr, nil},
		{"unsupported unescaped template key", invalidUnescUnsupKeyTplStr, errAny},
		{"unsupported unescaped template array", invalidUnescUnsupArrTplStr, errAny},
	}
	for _, tt := range ttYaml {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := CreateFromYamlTpl([]byte(tt.input))
			if tt.expErr == errAny {
				assert.Error(t, err)
			} else if !errors.Is(err, tt.expErr) {
				t.Errorf("expected error %v, got %v", tt.expErr, err)
			}
		})
	}
}
