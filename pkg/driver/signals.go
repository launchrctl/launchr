package driver

import (
	"context"
	"os"
	gosignal "os/signal"

	"github.com/moby/sys/signal"

	"github.com/launchrctl/launchr/internal/launchr"
)

// ForwardAllSignals forwards signals to the container
//
// The channel you pass in must already be setup to receive any signals you want to forward.
func ForwardAllSignals(ctx context.Context, cli ContainerRunner, cid string, sigc <-chan os.Signal) {
	var (
		s  os.Signal
		ok bool
	)
	for {
		select {
		case s, ok = <-sigc:
			if !ok {
				return
			}
		case <-ctx.Done():
			return
		}

		if s == signal.SIGCHLD || s == signal.SIGPIPE {
			continue
		}

		// In go1.14+, the go runtime issues SIGURG as an interrupt to support pre-emptable system calls on Linux.
		// Since we can't forward that along we'll check that here.
		if isRuntimeSig(s) {
			continue
		}
		var sig string
		for sigStr, sigN := range signal.SignalMap {
			if sigN == s {
				sig = sigStr
				break
			}
		}
		if sig == "" {
			continue
		}

		if err := cli.ContainerKill(ctx, cid, sig); err != nil {
			launchr.Log().Debug("error sending signal", "cid", cid, "error", err)
		}
	}
}

// NotifyAllSignals starts watching interrupt signals.
func NotifyAllSignals() chan os.Signal {
	sigc := make(chan os.Signal, 128)
	gosignal.Notify(sigc)
	return sigc
}

// StopCatchSignals stops catching the signals and closes the specified channel.
func StopCatchSignals(sigc chan os.Signal) {
	gosignal.Stop(sigc)
	close(sigc)
}
