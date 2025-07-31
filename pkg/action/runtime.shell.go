package action

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/launchrctl/launchr/internal/launchr"
)

type shellContext struct {
	Shell  string
	Env    []string
	Script string
}

type runtimeShell struct {
	WithLogger
}

// NewShellRuntime creates a new action shell runtime.
func NewShellRuntime() Runtime {
	return &runtimeShell{}
}

func (r *runtimeShell) Clone() Runtime {
	return NewShellRuntime()
}

func (r *runtimeShell) Init(_ context.Context, _ *Action) (err error) {
	return nil
}

func (r *runtimeShell) Execute(ctx context.Context, a *Action) (err error) {
	log := r.LogWith("run_env", "shell", "action_id", a.ID)
	log.Debug("starting execution of the action")

	streams := a.Input().Streams()
	shctx, err := createRTShellBashContext(a)
	if err != nil {
		return err
	}
	log.Debug("using shell", "shell", shctx.Shell, "env", shctx.Env, "script", shctx.Script)

	// Execute the script file directly
	cmd := exec.CommandContext(ctx, shctx.Shell, shctx.Script) //nolint:gosec // G204 user script is expected.
	cmd.Dir = a.WorkDir()
	cmd.Env = shctx.Env
	cmd.Stdout = streams.Out()
	cmd.Stderr = streams.Err()
	// Do no attach stdin, as it may not work as expected.

	err = cmd.Start()
	if err != nil {
		return err
	}
	log.Debug("started process", "pid", cmd.Process.Pid)

	// If we attached with TTY, all signals will be processed by a child process.
	sigc := launchr.NotifySignals()
	go launchr.HandleSignals(ctx, sigc, func(s os.Signal, _ string) error {
		log.Debug("forwarding signal for action", "sig", s, "pid", cmd.Process.Pid)
		return cmd.Process.Signal(s)
	})
	defer launchr.StopCatchSignals(sigc)

	cmdErr := cmd.Wait()
	var exitErr *exec.ExitError
	if errors.As(cmdErr, &exitErr) {
		exitCode := exitErr.ExitCode()
		msg := fmt.Sprintf("action %q finished with exit code %d", a.ID, exitCode)
		// Process was interrupted.
		if exitCode == -1 {
			exitCode = 130
			msg = fmt.Sprintf("action %q was interrupted, finished with exit code %d", a.ID, exitCode)
		}
		log.Info("action finished with exit code", "exit_code", exitCode)
		return launchr.NewExitError(exitCode, msg)
	}
	return cmdErr
}

func (r *runtimeShell) Close() error {
	return nil
}
