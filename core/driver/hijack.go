package driver

import (
	"context"
	"fmt"
	"io"
	"runtime"
	"sync"

	"github.com/moby/moby/api/types"
	"github.com/moby/moby/pkg/ioutils"
	"github.com/moby/moby/pkg/stdcopy"
	"github.com/moby/term"

	"github.com/launchrctl/launchr/core/cli"
	"github.com/launchrctl/launchr/core/log"
)

// The default escape key sequence: ctrl-p, ctrl-q
var defaultEscapeKeys = []byte{16, 17}

// ContainerInOut stores container driver in/out streams.
type ContainerInOut struct {
	In  io.WriteCloser
	Out io.Reader
}

// Close closes the hijacked connection and reader.
func (h *ContainerInOut) Close() error {
	return h.In.Close()
}

// CloseWrite closes a readWriter for writing.
func (h *ContainerInOut) CloseWrite() error {
	if conn, ok := h.In.(types.CloseWriter); ok {
		return conn.CloseWrite()
	}
	return nil
}

// Streamer is an interface for streaming in given in/out/err.
type Streamer interface {
	Stream(ctx context.Context) error
	Close() error
}

// ContainerIOStream streams in/out/err to given streams.
func ContainerIOStream(ctx context.Context, appCli cli.Streams, cio *ContainerInOut, config *ContainerCreateOptions) error {
	var (
		out, cerr io.Writer
		in        io.ReadCloser
	)
	if config.AttachStdin {
		in = appCli.In()
	}
	if config.AttachStdout {
		out = appCli.Out()
	}
	if config.AttachStderr {
		if config.Tty {
			cerr = appCli.Out()
		} else {
			cerr = appCli.Err()
		}
	}

	streamer := hijackedIOStreamer{
		streams:      appCli,
		inputStream:  in,
		outputStream: out,
		errorStream:  cerr,
		io:           cio,
		tty:          config.Tty,
	}

	errHijack := streamer.stream(ctx)
	return errHijack
}

type hijackedIOStreamer struct {
	streams      cli.Streams
	inputStream  io.ReadCloser
	outputStream io.Writer
	errorStream  io.Writer

	io *ContainerInOut

	tty        bool
	detachKeys string
}

func (h *hijackedIOStreamer) stream(ctx context.Context) error {
	restoreInput, err := h.setupInput()
	if err != nil {
		return fmt.Errorf("unable to setup input stream: %s", err)
	}

	defer restoreInput()

	outputDone := h.beginOutputStream(restoreInput)
	inputDone, detached := h.beginInputStream(restoreInput)

	select {
	case err := <-outputDone:
		return err
	case <-inputDone:
		// Input stream has closed.
		if h.outputStream != nil || h.errorStream != nil {
			// Wait for output to complete streaming.
			select {
			case err := <-outputDone:
				return err
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	case err := <-detached:
		// Got a detach key sequence.
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (h *hijackedIOStreamer) setupInput() (restore func(), err error) {
	if h.inputStream == nil || !h.tty {
		// No need to setup input TTY.
		// The restore func is a nop.
		return func() {}, nil
	}

	if err := setRawTerminal(h.streams); err != nil {
		return nil, fmt.Errorf("unable to set IO streams as raw terminal: %s", err)
	}

	// Use sync.Once so we may call restore multiple times but ensure we
	// only restore the terminal once.
	var restoreOnce sync.Once
	restore = func() {
		restoreOnce.Do(func() {
			_ = restoreTerminal(h.streams, h.inputStream)
		})
	}

	// Wrap the input to detect detach escape sequence.
	// Use default escape keys if an invalid sequence is given.
	escapeKeys := defaultEscapeKeys
	if h.detachKeys != "" {
		customEscapeKeys, err := term.ToBytes(h.detachKeys)
		if err != nil {
			log.Warn("invalid detach escape keys, using default: %s", err)
		} else {
			escapeKeys = customEscapeKeys
		}
	}

	h.inputStream = ioutils.NewReadCloserWrapper(term.NewEscapeProxy(h.inputStream, escapeKeys), h.inputStream.Close)

	return restore, nil
}

func (h *hijackedIOStreamer) beginOutputStream(restoreInput func()) <-chan error {
	if h.outputStream == nil && h.errorStream == nil {
		// There is no need to copy output.
		return nil
	}

	outputDone := make(chan error)
	go func() {
		var err error

		// When TTY is ON, use regular copy
		if h.outputStream != nil && h.tty {
			_, err = io.Copy(h.outputStream, h.io.Out)
			// We should restore the terminal as soon as possible
			// once the connection ends so any following print
			// messages will be in normal type.
			restoreInput()
		} else {
			_, err = stdcopy.StdCopy(h.outputStream, h.errorStream, h.io.Out)
		}

		log.Debug("[hijack] End of stdout")

		if err != nil {
			log.Debug("Error receiveStdout: %s", err)
		}

		outputDone <- err
	}()

	return outputDone
}

func (h *hijackedIOStreamer) beginInputStream(restoreInput func()) (doneC <-chan struct{}, detachedC <-chan error) {
	inputDone := make(chan struct{})
	detached := make(chan error)

	go func() {
		if h.inputStream != nil {
			_, err := io.Copy(h.io.In, h.inputStream)
			// We should restore the terminal as soon as possible
			// once the connection ends so any following print
			// messages will be in normal type.
			restoreInput()

			log.Debug("[hijack] End of stdin")
			if _, ok := err.(term.EscapeError); ok {
				detached <- err
				return
			}

			if err != nil {
				// This error will also occur on the receive
				// side (from stdout) where it will be
				// propagated back to the caller.
				log.Debug("Error sendStdin: %s", err)
			}
		}

		if err := h.io.CloseWrite(); err != nil {
			log.Debug("Couldn't send EOF: %s", err)
		}

		close(inputDone)
	}()

	return inputDone, detached
}

func setRawTerminal(streams cli.Streams) error {
	if err := streams.In().SetRawTerminal(); err != nil {
		return err
	}
	return streams.Out().SetRawTerminal()
}

func restoreTerminal(streams cli.Streams, in io.Closer) error {
	streams.In().RestoreTerminal()
	streams.Out().RestoreTerminal()
	// See github.com/docker/cli repo for more info.
	if in != nil && runtime.GOOS != "darwin" && runtime.GOOS != "windows" { //nolint:goconst
		return in.Close()
	}
	return nil
}
