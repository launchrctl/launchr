package cli

import (
	"testing"
	"testing/fstest"
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

	type testCase struct {
		name   string
		fs     fsmy
		expCfg bool
		expErr bool
	}

	tts := []testCase{
		{"valid config", fsmy{"config.yaml": validImgsYaml}, true, false},
		{"valid config 2", fsmy{"config.yml": validImgsYaml}, true, false},
		{"empty dir", fsmy{}, false, false},
		{"no config", fsmy{"config.yaml.bkp": "test", "my.config.yaml": "test"}, false, false},
		{"invalid config", fsmy{"config.yaml": invalidImgsYaml}, false, true},
	}
	for _, tt := range tts {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg, err := GlobalConfigFromDir(tt.fs.MapFS())
			if (err == nil) == tt.expErr {
				t.Errorf("unexpected error on config parsing")
			}
			if (cfg == nil) == tt.expCfg {
				t.Errorf("exected config result")
			}
		})
	}

}
