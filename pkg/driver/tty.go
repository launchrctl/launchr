package driver

import (
	"context"
	"time"

	"github.com/launchrctl/launchr/internal/launchr"
)

type terminalSize struct {
	Height uint
	Width  uint
}

type resizeTtyFn func(ctx context.Context, ropts terminalSize) error

// resizeTty is to resize the tty with cli out's tty size
func resizeTty(ctx context.Context, streams launchr.Streams, resizeFn resizeTtyFn) error {
	height, width := streams.Out().GetTtySize()
	if height == 0 && width == 0 {
		return nil
	}

	err := resizeFn(ctx, terminalSize{
		Height: height,
		Width:  width,
	})
	if err != nil {
		launchr.Log().Debug("error tty resize", "error", err)
	} else if errCtx := ctx.Err(); errCtx != nil {
		err = errCtx
	}
	return err
}

// initTtySize is to init the tty's size to the same as the window, if there is an error, it will retry 10 times.
func initTtySize(ctx context.Context, streams launchr.Streams, resizeFn resizeTtyFn) {
	if err := resizeTty(ctx, streams, resizeFn); err != nil {
		go func() {
			var err error
			for retry := 0; retry < 10; retry++ {
				time.Sleep(time.Duration(retry+1) * 10 * time.Millisecond)
				if err = resizeTty(ctx, streams, resizeFn); err == nil {
					break
				}
			}
			if err != nil {
				launchr.Log().Error("failed to resize tty, using default size", "err", streams.Err())
			}
		}()
	}
}

// TtySizeMonitor updates the container tty size when the terminal tty changes size
type TtySizeMonitor struct {
	resizeFn resizeTtyFn
}

// NewTtySizeMonitor creates a new TtySizeMonitor.
func NewTtySizeMonitor(resizeFn resizeTtyFn) *TtySizeMonitor {
	return &TtySizeMonitor{
		resizeFn: resizeFn,
	}
}

// Start starts tty size watching.
func (t *TtySizeMonitor) Start(ctx context.Context, streams launchr.Streams) {
	if t == nil {
		return
	}
	initTtySize(ctx, streams, t.resizeFn)
	watchTtySize(ctx, streams, t.resizeFn)
}
