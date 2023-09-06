package action

import (
	"bytes"
	"errors"
	"fmt"
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
		// Yaml action file is valid v1.
		{"valid yaml v1", validFullYaml, nil},
		// Valid, version is set implicitly to v1.
		{"valid empty version yaml v1", validEmptyVersionYaml, nil},
		// Version >v1 is unsupported.
		{"unsupported version >=1", unsupportedVersionYaml, errUnsupportedActionVersion{"2"}},

		// Image field in not provided v1.
		{"empty image field v1", invalidEmptyImgYaml, yamlTypeErrorLine(sErrEmptyActionImg, 4, 3)},
		{"empty string image field v1", invalidEmptyStrImgYaml, yamlTypeErrorLine(sErrEmptyActionImg, 6, 10)},
		// Command field in not provided v1.
		{"empty command field v1", invalidEmptyCmdYaml, yamlTypeErrorLine(sErrEmptyActionCmd, 4, 3)},
		{"empty array command field v1", invalidEmptyArrCmdYaml, yamlTypeErrorLine(sErrEmptyActionCmd, 6, 12)},

		// Arguments are incorrectly provided v1 - string, not an array of objects.
		{"invalid arguments field - string v1", invalidArgsStringYaml, yamlTypeErrorLine(sErrFieldMustBeArr, 5, 14)},
		// Arguments are incorrectly provided v1 - array of strings, not an array of objects.
		{"invalid arguments field - strings array", invalidArgsStringArrYaml, yamlTypeErrorLine(sErrArrElMustBeObj, 5, 15)},
		// Arguments are incorrectly provided v1 - object, not an array of objects.
		{"invalid arguments field - object", invalidArgsObjYaml, yamlTypeErrorLine(sErrFieldMustBeArr, 6, 5)},
		{"invalid argument empty name", invalidArgsEmptyNameYaml, yamlTypeErrorLine(sErrEmptyActionArgName, 6, 7)},
		{"invalid argument name", invalidArgsNameYaml, yamlTypeErrorLine(fmt.Sprintf(sErrInvalidActionArgName, "0arg"), 6, 13)},

		// Options are incorrectly provided v1 - string, not an array of objects.
		{"invalid options field - string", invalidOptsStrYaml, yamlTypeErrorLine(sErrFieldMustBeArr, 5, 12)},
		// Options are incorrectly provided v1 - array of strings, not an array of objects.
		{"invalid options field - string array", invalidOptsStrArrYaml, yamlTypeErrorLine(sErrArrElMustBeObj, 5, 13)},
		// Options are incorrectly provided v1 - object, not an array of objects.
		{"invalid options field - object", invalidOptsObjYaml, yamlTypeErrorLine(sErrFieldMustBeArr, 6, 5)},
		{"invalid option empty name", invalidOptsEmptyNameYaml, yamlTypeErrorLine(sErrEmptyActionOptName, 6, 7)},
		{"invalid option name", invalidOptsNameYaml, yamlTypeErrorLine(fmt.Sprintf(sErrInvalidActionOptName, "opt+name"), 6, 13)},
		{"invalid duplicate argument/option name", invalidDupArgsOptsNameYaml, yamlTypeErrorLine(fmt.Sprintf(sErrDupActionVarName, "dupName"), 8, 13)},
		{"invalid multiple errors", invalidMultipleErrYaml, yamlMergeErrors(
			yamlTypeErrorLine(fmt.Sprintf(sErrDupActionVarName, "dupName"), 8, 13),
			yamlTypeErrorLine(sErrEmptyActionOptName, 9, 7),
		)},
		{"invalid json schema type", invalidJsonSchemaTypeYaml, yamlTypeErrorLine(fmt.Sprintf("json schema type %q is unsupported", "unsup"), 7, 13)},
		{"invalid json schema default", invalidJsonSchemaDefaultYaml, yamlTypeErrorLine(fmt.Sprintf("value for json schema type %q is not implemented", "object"), 6, 7)},

		// Command declaration as array of strings.
		{"valid command - strings array", validCmdArrYaml, nil},
		{"invalid command - object", invalidCmdObjYaml, yamlTypeErrorLine(sErrArrOrStrEl, 6, 5)},
		{"invalid command - various array", invalidCmdArrVarYaml, yamlTypeErrorLine(sErrArrOrStrEl, 6, 5)},

		// Build image.
		{"build image - short", validBuildImgShortYaml, nil},
		{"build image - long", validBuildImgLongYaml, nil},

		// Extra hosts.
		{"extra hosts", validExtraHostsYaml, nil},
		{"extra hosts invalid", invalidExtraHostsYaml, yamlTypeErrorLine(sErrArrEl, 4, 16)},

		// Env variables replacement.
		{"env variables string array", validEnvArr, nil},
		{"env variables map", validEnvObj, nil},
		{"invalid env variables", invalidEnv, errAny},
		{"invalid env declaration - string", invalidEnvStr, yamlTypeErrorLine(sErrArrOrMapEl, 5, 8)},
		{"invalid env declaration - object", invalidEnvObj, yamlTypeErrorLine(sErrArrOrMapEl, 6, 5)},

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
			} else if assert.IsType(t, tt.expErr, err) {
				assert.Equal(t, tt.expErr, err)
			} else {
				assert.ErrorIs(t, tt.expErr, err)
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
			} else {
				assert.ErrorIs(t, tt.expErr, err)
			}
		})
	}
}
