package streams

import (
	"errors"
	"io"
	"os"

	"github.com/moby/term"
)

// In is an input stream used by the DockerCli to read user input
type In struct {
	commonStream
	in io.ReadCloser
}

func (i *In) Read(p []byte) (int, error) {
	return i.in.Read(p)
}

// Close implements the Closer interface
func (i *In) Close() error {
	return i.in.Close()
}

// SetRawTerminal sets raw mode on the input terminal
func (i *In) SetRawTerminal() (err error) {
	if os.Getenv("NORAW") != "" || !i.commonStream.isTerminal {
		return nil
	}
	i.commonStream.state, err = term.SetRawTerminal(i.commonStream.fd)
	return err
}

// CheckTty checks if we are trying to attach to a container tty
// from a non-tty client input stream, and if so, returns an error.
func (i *In) CheckTty(attachStdin, ttyMode bool) error {
	if ttyMode && attachStdin && !i.isTerminal {
		return errors.New("the input device is not a TTY")
	}
	return nil
}

// NewIn returns a new In object from a ReadCloser
func NewIn(in io.ReadCloser) *In {
	fd, isTerminal := term.GetFdInfo(in)
	return &In{commonStream: commonStream{fd: fd, isTerminal: isTerminal}, in: in}
}
