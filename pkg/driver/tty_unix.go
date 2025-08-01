//go:build unix

package driver

import (
	"context"
	"os"
	gosignal "os/signal"

	"github.com/moby/sys/signal"

	"github.com/launchrctl/launchr/internal/launchr"
)

func watchTtySize(ctx context.Context, streams launchr.Streams, resizeFn resizeTtyFn) {
	sigchan := make(chan os.Signal, 1)
	gosignal.Notify(sigchan, signal.SIGWINCH)
	go func() {
		defer gosignal.Stop(sigchan)
		for range sigchan {
			err := resizeTty(ctx, streams, resizeFn)
			if err != nil {
				// Stop monitoring
				return
			}
		}
	}()
}
