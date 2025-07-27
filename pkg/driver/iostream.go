package driver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"runtime"
	"sync"

	"github.com/launchrctl/launchr/internal/launchr"
)

// ContainerInOut stores container in/out streams.
type ContainerInOut struct {
	In  io.WriteCloser
	Out io.Reader
	Err io.Reader

	Opts       ContainerStreamsOptions
	TtyMonitor *TtySizeMonitor
}

// Close closes the IO and underlying connection.
func (cio *ContainerInOut) Close() error {
	return cio.In.Close()
}

// Stream streams in/out/err to given streams.
func (cio *ContainerInOut) Stream(ctx context.Context, streams launchr.Streams) error {
	var (
		out, cerr io.Writer
		in        io.ReadCloser
	)
	if cio.Opts.Stdin {
		in = streams.In()
	}
	if cio.Opts.Stdout {
		out = streams.Out()
	}
	if cio.Opts.Stderr {
		if cio.Opts.TTY {
			cerr = streams.Out()
		} else {
			cerr = streams.Err()
		}
	}

	streamer := ioStreamer{
		streams:      streams,
		inputStream:  in,
		outputStream: out,
		errorStream:  cerr,
		io:           cio,
		tty:          cio.Opts.TTY,
	}

	return streamer.stream(ctx)
}

type ioStreamer struct {
	streams      launchr.Streams
	inputStream  io.ReadCloser
	outputStream io.Writer
	errorStream  io.Writer

	io  *ContainerInOut
	tty bool
}

func (h *ioStreamer) stream(ctx context.Context) error {
	restoreInput, err := h.setupInput()
	if err != nil {
		return fmt.Errorf("unable to setup input stream: %s", err)
	}

	defer restoreInput()

	outputDone := h.beginOutputStream(restoreInput)
	inputDone := h.beginInputStream(restoreInput)

	// Close input.
	defer func() {
		if conn, ok := h.io.In.(interface{ CloseWrite() error }); ok {
			err := conn.CloseWrite()
			if err != nil {
				launchr.Log().Debug("couldn't send EOF", "error", err)
			}
		}
	}()

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
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (h *ioStreamer) setupInput() (restore func(), err error) {
	if h.inputStream == nil || !h.tty {
		// No need to setup input TTY.
		// The restore func is a nop.
		return func() {}, nil
	}

	if err := setRawTerminal(h.streams); err != nil {
		return nil, fmt.Errorf("unable to set io streams as raw terminal: %s", err)
	}

	// Use sync.Once so we may call restore multiple times but ensure we
	// only restore the terminal once.
	var restoreOnce sync.Once
	restore = func() {
		restoreOnce.Do(func() {
			_ = restoreTerminal(h.streams, h.inputStream)
		})
	}

	return restore, nil
}

func (h *ioStreamer) beginOutputStream(restoreInput func()) <-chan error {
	if h.outputStream == nil && h.errorStream == nil {
		// There is no need to copy output.
		return nil
	}

	outputDone := make(chan error)
	go func() {
		var err error

		// Copy streams.
		errChOut := h.copy(h.outputStream, h.io.Out)
		errChErr := h.copy(h.errorStream, h.io.Err)
		err = errors.Join(
			<-errChErr,
			<-errChOut,
		)
		// We should restore the terminal as soon as possible
		// once the connection ends so any following print
		// messages will be in normal type.
		restoreInput()

		if err != nil {
			launchr.Log().Debug("error receive stdout", "error", err)
		}
		launchr.Log().Debug("end of stdout/stderr")

		outputDone <- err
	}()

	return outputDone
}

func (h *ioStreamer) copy(dst io.Writer, src io.Reader) <-chan error {
	errChan := make(chan error)
	go func() {
		var err error
		if dst != nil && src != nil {
			_, err = io.Copy(dst, src)
		}
		errChan <- err
		close(errChan)
	}()
	return errChan
}

func (h *ioStreamer) beginInputStream(restoreInput func()) (doneC <-chan struct{}) {
	inputDone := make(chan struct{})

	go func() {
		if h.inputStream != nil {
			_, err := io.Copy(h.io.In, h.inputStream)
			// We should restore the terminal as soon as possible
			// once the connection ends so any following print
			// messages will be in normal type.
			restoreInput()

			launchr.Log().Debug("end of stdin")

			if err != nil {
				// This error will also occur on the receive
				// side (from stdout) where it will be
				// propagated back to the caller.
				launchr.Log().Debug("Error send Stdin", "error", err)
			}
		}

		close(inputDone)
	}()

	return inputDone
}

func setRawTerminal(streams launchr.Streams) error {
	if err := streams.In().SetRawTerminal(); err != nil {
		return err
	}
	return streams.Out().SetRawTerminal()
}

func restoreTerminal(streams launchr.Streams, in io.Closer) error {
	streams.In().RestoreTerminal()
	streams.Out().RestoreTerminal()
	// See github.com/docker/cli repo for more info.
	if in != nil && runtime.GOOS != "darwin" && runtime.GOOS != "windows" { //nolint:goconst
		return in.Close()
	}
	return nil
}
