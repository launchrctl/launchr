//go:build unix || windows

package test

import (
	"os"
	"strings"
	"syscall"
	_ "unsafe" // Include an internal method of the testscript module.

	"github.com/rogpeppe/go-internal/testscript"
)

func init() {
	tsScriptCmds["kill"] = CmdKill
}

var supportedKillSignals = map[string]syscall.Signal{
	"HUP":  syscall.SIGHUP,
	"INT":  syscall.SIGINT,
	"QUIT": syscall.SIGQUIT,
	"ILL":  syscall.SIGILL,
	"TRAP": syscall.SIGTRAP,
	"ABRT": syscall.SIGABRT,
	"BUS":  syscall.SIGBUS,
	"FPE":  syscall.SIGFPE,
	"KILL": syscall.SIGKILL,
	"SEGV": syscall.SIGSEGV,
	"PIPE": syscall.SIGPIPE,
	"ALRM": syscall.SIGALRM,
	"TERM": syscall.SIGTERM,
}

// CmdKill is an override of [github.com/rogpeppe/go-internal/testscript.(*TestScript).cmdKill].
// It supports more kill signals than the original.
func CmdKill(ts *testscript.TestScript, neg bool, args []string) {
	var (
		name   string
		signal os.Signal
	)
	switch len(args) {
	case 0:
	case 1, 2:
		sig, ok := strings.CutPrefix(args[0], "-")
		if ok {
			signal, ok = supportedKillSignals[sig]
			if !ok {
				ts.Fatalf("unknown signal: %s", sig)
			}
		} else {
			name = args[0]
			break
		}
		if len(args) == 2 {
			name = args[1]
		}
	default:
		ts.Fatalf("usage: kill [-SIGNAL] [name]")
	}
	if neg {
		ts.Fatalf("unsupported: ! kill")
	}
	if signal == nil {
		signal = os.Kill
	}
	if name != "" {
		killBackgroundOne(ts, name, signal)
	} else {
		killBackground(ts, signal)
	}
}

//go:linkname tsScriptCmds github.com/rogpeppe/go-internal/testscript.scriptCmds
var tsScriptCmds map[string]func(*testscript.TestScript, bool, []string)

//go:linkname killBackgroundOne github.com/rogpeppe/go-internal/testscript.(*TestScript).killBackgroundOne
func killBackgroundOne(ts *testscript.TestScript, bgName string, signal os.Signal)

//go:linkname killBackground github.com/rogpeppe/go-internal/testscript.(*TestScript).killBackground
func killBackground(ts *testscript.TestScript, signal os.Signal)
