//go:build windows

package driver

import (
	"context"

	"github.com/launchrctl/launchr/internal/launchr"
)

func watchTtySize(ctx context.Context, streams launchr.Streams, resizeFn resizeTtyFn) {
	go func() {
		prevH, prevW := streams.Out().GetTtySize()
		for {
			h, w := streams.Out().GetTtySize()

			if prevW != w || prevH != h {
				err := resizeTty(ctx, streams, resizeFn)
				if err != nil {
					// Stop monitoring
					return
				}
			}
			prevH = h
			prevW = w
		}
	}()
}
