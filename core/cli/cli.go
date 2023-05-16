package cli

import (
	"bytes"
	"io"
	"io/fs"
	"path"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/moby/term"

	"github.com/launchrctl/launchr/core/cli/streams"
)

// AppCliOption applies a modification on a DockerCli.
type AppCliOption func(cli *AppCli) error

// Streams is an interface which exposes the standard input and output streams
type Streams interface {
	In() *streams.In
	Out() *streams.Out
	Err() io.Writer
}

// Cli is an interface for app common cli.
type Cli interface {
	In() *streams.In
	Out() *streams.Out
	Err() io.Writer
	Config() *GlobalConfig
}

// AppCli is a global app config.
type AppCli struct {
	in  *streams.In
	out *streams.Out
	err io.Writer
	cfg *GlobalConfig
}

// NewAppCli creates AppCli configuration to be used in output for actions.
func NewAppCli(ops ...AppCliOption) (Cli, error) {
	defaultOps := []AppCliOption{}
	ops = append(defaultOps, ops...)
	cli := &AppCli{
		cfg: &GlobalConfig{},
	}
	if err := cli.Apply(ops...); err != nil {
		return nil, err
	}
	return cli, nil
}

// In returns the reader used for stdin
func (cli *AppCli) In() *streams.In {
	return cli.in
}

// Out returns the writer used for stdout
func (cli *AppCli) Out() *streams.Out {
	return cli.out
}

// Err returns the writer used for stderr
func (cli *AppCli) Err() io.Writer {
	return cli.err
}

// Config returns cli global configuration.
func (cli *AppCli) Config() *GlobalConfig {
	return cli.cfg
}

// Apply all the operation on the cli
func (cli *AppCli) Apply(ops ...AppCliOption) error {
	for _, op := range ops {
		if err := op(cli); err != nil {
			return err
		}
	}
	return nil
}

// WithStandardStreams sets a cli in, out and err streams with the standard streams.
func WithStandardStreams() AppCliOption {
	return func(cli *AppCli) error {
		// Set terminal emulation based on platform as required.
		stdin, stdout, stderr := term.StdStreams()
		cli.in = streams.NewIn(stdin)
		cli.out = streams.NewOut(stdout)
		cli.err = stderr
		return nil
	}
}

// WithFakeStreams provides fake in/out/err streams.
func WithFakeStreams() AppCliOption {
	return func(cli *AppCli) error {
		outBuffer := new(bytes.Buffer)
		errBuffer := new(bytes.Buffer)
		cli.in = streams.NewIn(io.NopCloser(strings.NewReader("")))
		cli.out = streams.NewOut(outBuffer)
		cli.err = errBuffer
		return nil
	}
}

// WithGlobalConfigFromDir is a hook to initialize global config.
func WithGlobalConfigFromDir(dirRoot fs.FS) AppCliOption {
	return func(cli *AppCli) (err error) {
		cli.cfg, err = GlobalConfigFromDir(dirRoot)
		return err
	}
}

// GetFsAbsPath returns absolute path for a FS struct.
func GetFsAbsPath(fs fs.FS) string {
	cwd := ""
	fsRefl := reflect.ValueOf(fs)
	if fsRefl.Kind() == reflect.String {
		var err error
		cwd = fsRefl.String()
		// @todo Rethink absolute path usage overall.
		if !path.IsAbs(cwd) {
			cwd, err = filepath.Abs(cwd)
			if err != nil {
				panic("can't retrieve absolute path for the path")
			}
		}
	}
	return cwd
}
