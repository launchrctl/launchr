package cli

import (
	"bytes"
	"io"
	"strings"

	"github.com/moby/term"
)

type appCli struct {
	in  *In
	out *Out
	err io.Writer
}

func (cli *appCli) In() *In {
	return cli.in
}

func (cli *appCli) Out() *Out {
	return cli.out
}

func (cli *appCli) Err() io.Writer {
	return cli.err
}

// StandardStreams sets a cli in, out and err streams with the standard streams.
func StandardStreams() Streams {
	// Set terminal emulation based on platform as required.
	stdin, stdout, stderr := term.StdStreams()
	return &appCli{
		in:  NewIn(stdin),
		out: NewOut(stdout),
		err: stderr,
	}
}

// InMemoryStreams provides in-memory in/out/err streams.
func InMemoryStreams() Streams {
	outBuffer := &bytes.Buffer{}
	errBuffer := &bytes.Buffer{}
	return &appCli{
		in:  NewIn(io.NopCloser(strings.NewReader(""))),
		out: NewOut(outBuffer),
		err: errBuffer,
	}
}
