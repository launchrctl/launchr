package launchr

import (
	"errors"
	"io"

	mobyterm "github.com/moby/term"
)

// Streams is an interface which exposes the standard input and output streams.
type Streams interface {
	// In returns the reader used for stdin.
	In() *In
	// Out returns the writer used for stdout.
	Out() *Out
	// Err returns the writer used for stderr.
	Err() *Out
	io.Closer
}

type commonStream struct {
	fd    uintptr
	state *mobyterm.State

	isDiscard  bool
	isTerminal bool
}

// FD returns the file descriptor number for this stream.
func (s *commonStream) FD() uintptr {
	return s.fd
}

// IsDiscard returns if read/write is discarded.
func (s *commonStream) IsDiscard() bool {
	return s.isDiscard
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
// Used for tests only.
func (s *commonStream) SetIsTerminal(isTerminal bool) {
	s.isTerminal = isTerminal
}

// Out is an output stream used by the app to write normal program output.
type Out struct {
	commonStream
	out io.Writer
}

func (o *Out) Write(p []byte) (int, error) {
	if o.out == nil {
		// Discard.
		return len(p), nil
	}
	return o.out.Write(p)
}

// SetRawTerminal sets raw mode on the input terminal.
func (o *Out) SetRawTerminal() (err error) {
	if !o.commonStream.IsTerminal() {
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

// Writer returns the wrapped writer.
func (o *Out) Writer() io.Writer {
	return o.out
}

// Close implement [io.Closer]
func (o *Out) Close() error {
	if out, ok := o.out.(io.Closer); ok {
		return out.Close()
	}
	return nil
}

// NewOut returns a new [Out] object from an [io.Writer].
func NewOut(out io.Writer) *Out {
	fd, isTerminal := mobyterm.GetFdInfo(out)
	isDiscard := out == nil
	return &Out{
		commonStream: commonStream{
			fd:         fd,
			isTerminal: isTerminal,
			isDiscard:  isDiscard,
		},
		out: out,
	}
}

// In is an input stream used by the app to read user input.
type In struct {
	commonStream
	in io.ReadCloser
}

func (i *In) Read(p []byte) (int, error) {
	if i.in == nil {
		// Discard.
		return 0, io.EOF
	}
	return i.in.Read(p)
}

// Close implements the [io.Closer] interface.
func (i *In) Close() error {
	if i.in == nil {
		return nil
	}
	return i.in.Close()
}

// SetRawTerminal sets raw mode on the input terminal.
func (i *In) SetRawTerminal() (err error) {
	if !i.commonStream.IsTerminal() {
		return nil
	}
	i.commonStream.state, err = mobyterm.SetRawTerminal(i.commonStream.fd)
	return err
}

// Reader returns the wrapped reader.
func (i *In) Reader() io.ReadCloser {
	return i.in
}

// NewIn returns a new [In] object from a [io.ReadCloser]
func NewIn(in io.ReadCloser) *In {
	fd, isTerminal := mobyterm.GetFdInfo(in)
	isDiscard := in == nil
	return &In{
		commonStream: commonStream{
			fd:         fd,
			isTerminal: isTerminal,
			isDiscard:  isDiscard,
		},
		in: in,
	}
}

type appCli struct {
	in  *In
	out *Out
	err *Out
}

func (cli *appCli) In() *In   { return cli.in }
func (cli *appCli) Out() *Out { return cli.out }
func (cli *appCli) Err() *Out { return cli.err }

func (cli *appCli) Close() error {
	return errors.Join(
		cli.in.Close(),
		cli.out.Close(),
		cli.err.Close(),
	)
}

// NewBasicStreams creates streams with given in, out and err streams.
// Give decorate functions to extend functionality.
func NewBasicStreams(in io.ReadCloser, out io.Writer, err io.Writer, fns ...StreamsModifierFn) Streams {
	streams := &appCli{
		in:  NewIn(in),
		out: NewOut(out),
		err: NewOut(err),
	}
	for _, fn := range fns {
		fn(streams)
	}
	return streams
}

// MaskedStdStreams sets a cli in, out and err streams with the standard streams and with masking of sensitive data.
func MaskedStdStreams(mask *SensitiveMask) Streams {
	stdin, stdout, stderr := StdInOutErr()
	return NewBasicStreams(stdin, stdout, stderr, WithSensitiveMask(mask))
}

// StdInOutErr returns the standard streams (stdin, stdout, stderr).
//
// On Windows, it attempts to turn on VT handling on all std handles if
// supported, or falls back to terminal emulation. On Unix, this returns
// the standard [os.Stdin], [os.Stdout] and [os.Stderr].
func StdInOutErr() (stdIn io.ReadCloser, stdOut, stdErr io.Writer) {
	return mobyterm.StdStreams()
}

// NoopStreams provides streams like /dev/null.
func NoopStreams() Streams {
	return NewBasicStreams(nil, nil, nil)
}

// StreamsModifierFn is a decorator function for a stream.
type StreamsModifierFn func(streams *appCli)

// WithSensitiveMask decorates streams with a given mask.
func WithSensitiveMask(m *SensitiveMask) StreamsModifierFn {
	return func(streams *appCli) {
		streams.out.out = NewMaskingWriter(streams.out.out, m)
		streams.err.out = NewMaskingWriter(streams.err.out, m)
	}
}
