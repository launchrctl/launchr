package action

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"syscall"

	"github.com/launchrctl/launchr/internal/launchr"
)

type runtimeShell struct {
}

// NewShellRuntime creates a new action shell runtime.
func NewShellRuntime() Runtime {
	return &runtimeShell{}
}

func (r *runtimeShell) Clone() Runtime {
	return NewShellRuntime()
}

func (r *runtimeShell) Init(_ context.Context, _ *Action) (err error) {
	if runtime.GOOS == "windows" {
		return fmt.Errorf("shell runtime is not supported in Windows")
	}
	return nil
}

func (r *runtimeShell) Execute(ctx context.Context, a *Action) (err error) {
	streams := a.Input().Streams()
	rt := a.RuntimeDef()
	defaultShell := os.Getenv("SHELL")
	if defaultShell == "" {
		defaultShell = "/bin/bash"
	}

	cmd := exec.CommandContext(ctx, defaultShell, "-l", "-c", rt.Shell.Script) //nolint:gosec // G204 user script is expected.
	cmd.Dir = a.WorkDir()
	cmd.Env = append(os.Environ(), rt.Shell.Env...)
	cmd.Stdout = streams.Out()
	// Attach stdin.
	if streams.In().IsTerminal() {
		// Pass file directly because [CommandContext] does explicit checks in case of stdin.
		// If stdin is not passed directly, it will block forever waiting for input.
		cmd.Stdin = streams.In().Reader()
		// Create a new process group for the subprocess, so signals don't propagate
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setpgid: true,
		}
	}
	cmd.Stderr = streams.Err()

	err = cmd.Start()
	if err != nil {
		return err
	}

	// If we attached with TTY, all signals will be processed by a child process.
	sigc := launchr.NotifySignals()
	go launchr.HandleSignals(ctx, sigc, func(s os.Signal, _ string) error {
		launchr.Log().Debug("forwarding signal for action", "signal", s, "action", a.ID)
		return syscall.Kill(-cmd.Process.Pid, s.(syscall.Signal))
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
		return launchr.NewExitError(exitCode, msg)
	}
	return cmdErr
}

func (r *runtimeShell) Close() error {
	return nil
}
