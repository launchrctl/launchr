package driver

import (
	"context"
	"fmt"
	"os"
	gosignal "os/signal"
	"runtime"
	"time"

	"github.com/moby/sys/signal"

	"github.com/launchrctl/launchr/pkg/cli"
	"github.com/launchrctl/launchr/pkg/log"
	"github.com/launchrctl/launchr/pkg/types"
)

type resizeTtyFn func(ctx context.Context, d ContainerRunner, cli cli.Streams, id string, isExec bool) error

// resizeTtyTo resizes tty to specific height and width
func resizeTtyTo(ctx context.Context, d ContainerRunner, id string, height, width uint, isExec bool) error {
	if height == 0 && width == 0 {
		return nil
	}

	options := types.ResizeOptions{
		Height: height,
		Width:  width,
	}

	var err error
	if isExec {
		err = d.ContainerExecResize(ctx, id, options)
	} else {
		err = d.ContainerResize(ctx, id, options)
	}

	if err != nil {
		log.Debug("Error resize: %s\r", err)
	}
	return err
}

// resizeTty is to resize the tty with cli out's tty size
func resizeTty(ctx context.Context, d ContainerRunner, cli cli.Streams, id string, isExec bool) error {
	height, width := cli.Out().GetTtySize()
	return resizeTtyTo(ctx, d, id, height, width, isExec)
}

// initTtySize is to init the tty's size to the same as the window, if there is an error, it will retry 10 times.
func initTtySize(ctx context.Context, d ContainerRunner, cli cli.Streams, id string, isExec bool, resizeTtyFunc resizeTtyFn) {
	rttyFunc := resizeTtyFunc
	if rttyFunc == nil {
		rttyFunc = resizeTty
	}
	if err := rttyFunc(ctx, d, cli, id, isExec); err != nil {
		go func() {
			var err error
			for retry := 0; retry < 10; retry++ {
				time.Sleep(time.Duration(retry+1) * 10 * time.Millisecond)
				if err = rttyFunc(ctx, d, cli, id, isExec); err == nil {
					break
				}
			}
			if err != nil {
				fmt.Fprintln(cli.Err(), "failed to resize tty, using default size")
			}
		}()
	}
}

// MonitorTtySize updates the container tty size when the terminal tty changes size
func MonitorTtySize(ctx context.Context, d ContainerRunner, cli cli.Streams, id string, isExec bool) error {
	initTtySize(ctx, d, cli, id, isExec, resizeTty)
	if runtime.GOOS == "windows" {
		go func() {
			prevH, prevW := cli.Out().GetTtySize()
			for {
				time.Sleep(time.Millisecond * 250)
				h, w := cli.Out().GetTtySize()

				if prevW != w || prevH != h {
					err := resizeTty(ctx, d, cli, id, isExec)
					if err != nil {
						// Stop monitoring
						return
					}
				}
				prevH = h
				prevW = w
			}
		}()
	} else {
		sigchan := make(chan os.Signal, 1)
		gosignal.Notify(sigchan, signal.SIGWINCH)
		go func() {
			for range sigchan {
				err := resizeTty(ctx, d, cli, id, isExec)
				if err != nil {
					// Stop monitoring
					return
				}
			}
		}()
	}
	return nil
}
