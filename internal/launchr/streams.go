package launchr

import (
	"errors"
	"io"
	"os"
	"strings"

	mobyterm "github.com/moby/term"
)

// Streams is an interface which exposes the standard input and output streams.
type Streams interface {
	// In returns the reader used for stdin.
	In() *In
	// Out returns the writer used for stdout.
	Out() *Out
	// Err returns the writer used for stderr.
	Err() io.Writer
}

type commonStream struct {
	fd         uintptr
	isTerminal bool
	state      *mobyterm.State
}

// FD returns the file descriptor number for this stream.
func (s *commonStream) FD() uintptr {
	return s.fd
}

// IsTerminal returns true if this stream is connected to a terminal.
func (s *commonStream) IsTerminal() bool {
	return s.isTerminal
}

// RestoreTerminal restores normal mode to the terminal.
func (s *commonStream) RestoreTerminal() {
	if s.state != nil {
		_ = mobyterm.RestoreTerminal(s.fd, s.state)
	}
}

// SetIsTerminal sets the boolean used for isTerminal.
func (s *commonStream) SetIsTerminal(isTerminal bool) {
	s.isTerminal = isTerminal
}

// Out is an output stream used by the app to write normal program output.
type Out struct {
	commonStream
	out io.Writer
}

func (o *Out) Write(p []byte) (int, error) {
	return o.out.Write(p)
}

// SetRawTerminal sets raw mode on the input terminal.
func (o *Out) SetRawTerminal() (err error) {
	if os.Getenv("NORAW") != "" || !o.commonStream.isTerminal {
		return nil
	}
	o.commonStream.state, err = mobyterm.SetRawTerminalOutput(o.commonStream.fd)
	return err
}

// GetTtySize returns the height and width in characters of the tty.
func (o *Out) GetTtySize() (uint, uint) {
	if !o.isTerminal {
		return 0, 0
	}
	ws, err := mobyterm.GetWinsize(o.fd)
	if err != nil {
		Log().Debug("error getting tty size", "error", err)
		if ws == nil {
			return 0, 0
		}
	}
	return uint(ws.Height), uint(ws.Width)
}

// NewOut returns a new [Out] object from a [io.Writer].
func NewOut(out io.Writer) *Out {
	fd, isTerminal := mobyterm.GetFdInfo(out)
	return &Out{commonStream: commonStream{fd: fd, isTerminal: isTerminal}, out: out}
}

// In is an input stream used by the app to read user input.
type In struct {
	commonStream
	in io.ReadCloser
}

func (i *In) Read(p []byte) (int, error) {
	return i.in.Read(p)
}

// Close implements the [io.Closer] interface.
func (i *In) Close() error {
	return i.in.Close()
}

// SetRawTerminal sets raw mode on the input terminal.
func (i *In) SetRawTerminal() (err error) {
	if os.Getenv("NORAW") != "" || !i.commonStream.isTerminal {
		return nil
	}
	i.commonStream.state, err = mobyterm.SetRawTerminal(i.commonStream.fd)
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

// NewIn returns a new [In] object from a [io.ReadCloser]
func NewIn(in io.ReadCloser) *In {
	fd, isTerminal := mobyterm.GetFdInfo(in)
	return &In{commonStream: commonStream{fd: fd, isTerminal: isTerminal}, in: in}
}

type appCli struct {
	in  *In
	out *Out
	err io.Writer
}

func (cli *appCli) In() *In        { return cli.in }
func (cli *appCli) Out() *Out      { return cli.out }
func (cli *appCli) Err() io.Writer { return cli.err }

// StandardStreams sets a cli in, out and err streams with the standard streams.
func StandardStreams() Streams {
	// Set terminal emulation based on platform as required.
	stdin, stdout, stderr := mobyterm.StdStreams()
	return &appCli{
		in:  NewIn(stdin),
		out: NewOut(stdout),
		err: stderr,
	}
}

// NoopStreams provides streams like /dev/null.
func NoopStreams() Streams {
	return &appCli{
		in:  NewIn(io.NopCloser(strings.NewReader(""))),
		out: NewOut(io.Discard),
		err: io.Discard,
	}
}
