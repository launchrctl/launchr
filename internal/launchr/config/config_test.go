package config

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

func Test_GlobalConfigFromDir(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	type testYamlFieldSub struct {
		Field1 string `yaml:"field1"`
		Field2 int    `yaml:"field2"`
	}

	type expValType struct {
		stru    testYamlFieldSub
		struErr string
		ptr     *testYamlFieldSub
		ptrErr  string
		prim    testYamlFieldSub
	}
	type testCase struct {
		name   string
		fs     fsmy
		expVal expValType
	}

	expValid := expValType{
		stru: testYamlFieldSub{"str", 1},
		ptr:  &testYamlFieldSub{"str3", 3},
		prim: testYamlFieldSub{"str2", 2},
	}
	var expEmpty expValType
	expInvalid := expValType{
		struErr: "yaml: unmarshal errors",
	}
	var errCheck = func(err error, errStr string) {
		if errStr == "" {
			assert.NoError(err)
		} else {
			assert.ErrorContains(err, errStr)
		}
	}

	// @todo test multiple structs
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
			cfg := GlobalConfigFromFS(tt.fs.MapFS())
			assert.NotNil(cfg)
			var err error
			var val1, val1c testYamlFieldSub
			var val1ptr *testYamlFieldSub
			// Check struct.
			err = cfg.Get("test_perm", &val1)
			errCheck(err, tt.expVal.struErr)
			assert.Equal(tt.expVal.stru, val1)
			// Check cache works.
			err = cfg.Get("test_perm", &val1c)
			errCheck(err, tt.expVal.struErr)
			assert.Equal(val1, val1c)
			// Check pointer to a struct.
			err = cfg.Get("test_ptr", &val1ptr)
			errCheck(err, tt.expVal.ptrErr)
			assert.Equal(tt.expVal.ptr, val1ptr)

			// Check primitives.
			var val2s string
			var val2int int
			_ = cfg.Get("field1", &val2s)
			_ = cfg.Get("field2", &val2int)
			assert.Equal(tt.expVal.prim.Field1, val2s)
			assert.Equal(tt.expVal.prim.Field2, val2int)
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
`

const yamlInvalid = `
test_perm:
  field1: str
  field2: [1, 2]
field2: [3, 4]
`
