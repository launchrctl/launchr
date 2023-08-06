package cli

import (
	"bytes"
	"io"
	"strings"

	"github.com/moby/term"
)

// AppCli is a global app config.
type AppCli struct {
	in  *In
	out *Out
	err io.Writer
}

// In returns the reader used for stdin
func (cli *AppCli) In() *In {
	return cli.in
}

// Out returns the writer used for stdout
func (cli *AppCli) Out() *Out {
	return cli.out
}

// Err returns the writer used for stderr
func (cli *AppCli) Err() io.Writer {
	return cli.err
}

// StandardStreams sets a cli in, out and err streams with the standard streams.
func StandardStreams() Streams {
	// Set terminal emulation based on platform as required.
	stdin, stdout, stderr := term.StdStreams()
	return &AppCli{
		in:  NewIn(stdin),
		out: NewOut(stdout),
		err: stderr,
	}
}

// InMemoryStreams provides in-memory in/out/err streams.
func InMemoryStreams() Streams {
	outBuffer := &bytes.Buffer{}
	errBuffer := &bytes.Buffer{}
	return &AppCli{
		in:  NewIn(io.NopCloser(strings.NewReader(""))),
		out: NewOut(outBuffer),
		err: errBuffer,
	}
}
