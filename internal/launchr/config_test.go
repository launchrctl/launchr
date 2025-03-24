package launchr

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
)

type fsmy map[string]string

func (f fsmy) MapFS() fstest.MapFS {
	m := make(fstest.MapFS)
	for k, v := range f {
		m[k] = &fstest.MapFile{Data: []byte(v)}
	}
	return m
}

func Test_ConfigFromFS(t *testing.T) {
	t.Parallel()

	type testYamlFieldSub struct {
		Field1 string `yaml:"field1"`
		Field2 int    `yaml:"field2"`
	}

	type testYamlCustomTag struct {
		CustomTag []testYamlFieldSub `yaml:"custom_tag"`
	}

	type expValType struct {
		Struct       testYamlFieldSub
		StructErr    string
		CustomTag    testYamlCustomTag
		CustomTagErr string
		Ptr          *testYamlFieldSub
		PtrErr       string
		Primitive    testYamlFieldSub
	}
	type testCase struct {
		name   string
		fs     fsmy
		expVal expValType
	}

	expValid := expValType{
		Struct: testYamlFieldSub{"str", 1},
		CustomTag: testYamlCustomTag{
			[]testYamlFieldSub{{"str4", 4}},
		},
		Ptr:       &testYamlFieldSub{"str3", 3},
		Primitive: testYamlFieldSub{"str2", 2},
	}
	var expEmpty expValType
	expInvalid := expValType{
		StructErr: "error(s) decoding",
	}
	var errCheck = func(t *testing.T, err error, errStr string) {
		if errStr == "" {
			assert.NoError(t, err)
		} else {
			assert.ErrorContains(t, err, errStr)
		}
	}

	// @todo test parsed yaml for struct fields.
	tts := []testCase{
		{"valid config yaml", fsmy{"config.yaml": yamlValid}, expValid},
		{"valid empty config yml", fsmy{"config.yml": ""}, expEmpty},
		{"unknown data", fsmy{"config.yml": "na: str"}, expEmpty},
		{"empty dir", fsmy{}, expEmpty},
		{"no config", fsmy{"config.yaml.bkp": yamlValid, "my.config.yaml": yamlValid}, expEmpty},
		{"invalid config", fsmy{"config.yaml": yamlInvalid}, expInvalid},
	}
	for _, tt := range tts {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := ConfigFromFS(tt.fs.MapFS())
			assert.NotNil(t, cfg)
			var err error
			var val1, val1c testYamlFieldSub
			var valcustom testYamlCustomTag
			var val1ptr *testYamlFieldSub
			// Check struct.
			err = cfg.Get("test_perm", &val1)
			errCheck(t, err, tt.expVal.StructErr)
			assert.Equal(t, tt.expVal.Struct, val1)
			// Check custom.
			err = cfg.Get("test_custom", &valcustom)
			errCheck(t, err, tt.expVal.CustomTagErr)
			assert.Equal(t, tt.expVal.CustomTag, valcustom)

			// Check cache works.
			err = cfg.Get("test_perm", &val1c)
			errCheck(t, err, tt.expVal.StructErr)
			assert.Equal(t, val1, val1c)
			// Check pointer to a struct.
			err = cfg.Get("test_ptr", &val1ptr)
			errCheck(t, err, tt.expVal.PtrErr)
			assert.Equal(t, tt.expVal.Ptr, val1ptr)

			// Check primitives.
			var val2s string
			var val2int int
			_ = cfg.Get("field1", &val2s)
			_ = cfg.Get("field2", &val2int)
			assert.Equal(t, tt.expVal.Primitive.Field1, val2s)
			assert.Equal(t, tt.expVal.Primitive.Field2, val2int)
		})
	}
}

const yamlValid = `
field1: str2
field2: 2
test_perm:
  field1: str
  field2: 1
test_ptr:
  field1: str3
  field2: 3
test_custom:
  custom_tag:
    - field1: str4
      field2: 4
`

const yamlInvalid = `
test_perm:
  field1: str
  field2: [1, 2]
field2: [3, 4]
`
