package launchr

import (
	"context"
	"os"
	gosignal "os/signal"

	"github.com/moby/sys/signal"
)

// HandleSignals forwards signals to the handler.
func HandleSignals(ctx context.Context, sigc <-chan os.Signal, killFn func(s os.Signal, sig string) error) {
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

		if err := killFn(s, sig); err != nil {
			Log().Debug("error sending signal", "error", err, "sig", sig)
		}
	}
}

// NotifySignals starts watching interrupt signals.
func NotifySignals(sig ...os.Signal) chan os.Signal {
	sigc := make(chan os.Signal, 128)
	gosignal.Notify(sigc, sig...)
	return sigc
}

// StopCatchSignals stops catching the signals and closes the specified channel.
func StopCatchSignals(sigc chan os.Signal) {
	gosignal.Stop(sigc)
	close(sigc)
}
