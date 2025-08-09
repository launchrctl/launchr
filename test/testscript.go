// Package test contains functionality to test the application with testscript.
package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/rogpeppe/go-internal/testscript"

	"github.com/launchrctl/launchr/internal/launchr"
	_ "github.com/launchrctl/launchr/test/plugins" // Include test plugins.
)

// CmdsTestScript provides custom commands for testscript execution.
func CmdsTestScript() map[string]func(ts *testscript.TestScript, neg bool, args []string) {
	return map[string]func(ts *testscript.TestScript, neg bool, args []string){
		// txtproc provides flexible text processing capabilities
		// Usage:
		//	txtproc replace 'old' 'new' input.txt output.txt
		//	txtproc replace-regex 'pattern' 'replacement' input.txt output.txt
		//	txtproc remove-lines 'pattern' input.txt output.txt
		//	txtproc remove-regex 'pattern' input.txt output.txt
		//	txtproc extract-lines 'pattern' input.txt output.txt
		//	txtproc extract-regex 'pattern' input.txt output.txt
		"txtproc": CmdTxtProc,
		// sleep pauses execution for a specified duration
		// Usage:
		//  sleep <duration>
		// Examples:
		//	sleep 1s
		//	sleep 500ms
		//	sleep 2m
		"sleep": CmdSleep,
		// dlv runs the given binary with Delve for debugging.
		// Please, note that the test must be run with debug headers for it to work.
		// Usage:
		//   dlv <app_name>
		"dlv": CmdDlv,
	}
}

// SetupEnvDocker configures docker backend in the test environment.
func SetupEnvDocker(env *testscript.Env) error {
	env.Vars = append(
		env.Vars,
		// Passthrough Docker env variables if set.
		"DOCKER_HOST="+os.Getenv("DOCKER_HOST"),
		"DOCKER_TLS_VERIFY="+os.Getenv("DOCKER_TLS_VERIFY"),
		"DOCKER_CERT_PATH="+os.Getenv("DOCKER_CERT_PATH"),
	)
	return nil
}

// SetupEnvRandom sets up a random environment variable.
func SetupEnvRandom(env *testscript.Env) error {
	env.Vars = append(
		env.Vars,
		"RANDOM="+launchr.GetRandomString(8),
	)
	return nil
}

// SetupWorkDirUnixWin sets up a work dir env variable in unix style.
func SetupWorkDirUnixWin(env *testscript.Env) error {
	env.Vars = append(
		env.Vars,
		"WORK_UNIX="+launchr.ConvertWindowsPath(env.WorkDir),
	)
	return nil
}

// SetupWSL sets up a work dir env variable using WSL mount path.
func SetupWSL(t *testing.T) func(env *testscript.Env) error {
	return func(env *testscript.Env) error {
		// Take WSL script path from env variable.
		wslBashPath := os.Getenv("TEST_WSL_BASH_PATH")
		// Try to create a wrapper for WSL.
		if wslBashPath == "" {
			wslpath, err := exec.LookPath("wsl")
			if err != nil {
				panic(err)
			}
			content := "@echo off\r\n" + wslpath + " bash %*"

			// Create the file
			wslBashPath = filepath.Join(t.TempDir(), "wsl-bash.cmd")
			file, err := os.Create(wslBashPath) //nolint:gosec // G304 We create the path.
			if err != nil {
				panic(err)
			}

			// Write the content to the file
			_, err = file.WriteString(content)
			if err != nil {
				_ = file.Close()
				panic(err)
			}
			_ = file.Close()
		}
		env.Vars = append(
			env.Vars,
			"WORK_UNIX=/mnt"+launchr.ConvertWindowsPath(env.WorkDir),
			"LAUNCHR_RUNTIME_SHELL_BASH="+wslBashPath,
		)
		return nil
	}
}

// CmdSleep pauses execution for a specified duration
func CmdSleep(ts *testscript.TestScript, neg bool, args []string) {
	if neg {
		ts.Fatalf("sleep does not support negation")
	}

	if len(args) != 1 {
		ts.Fatalf("sleep: usage: sleep <duration>")
	}

	duration, err := time.ParseDuration(args[0])
	if err != nil {
		// Try parsing as seconds if it's just a number
		if seconds, numErr := strconv.ParseFloat(args[0], 64); numErr == nil {
			duration = time.Duration(seconds * float64(time.Second))
		} else {
			ts.Fatalf("sleep: invalid duration %q: %v", args[0], err)
		}
	}

	if duration < 0 {
		ts.Fatalf("sleep: duration cannot be negative")
	}

	time.Sleep(duration)
}
