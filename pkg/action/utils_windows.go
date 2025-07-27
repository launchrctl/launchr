//go:build windows

package action

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/launchrctl/launchr/internal/launchr"
)

func getCurrentUser() userInfo {
	// Use neutral 1000 when we can't get UID on Windows.
	const defaultUID = "1000"
	const defaultGID = "1000"
	return userInfo{
		UID: defaultUID,
		GID: defaultGID,
	}
}

func normalizeContainerMountPath(path string) string {
	path = launchr.MustAbs(path)
	// Convert windows paths C:\my\path -> /c/my/path for docker daemon.
	return "/mnt" + launchr.ConvertWindowsPath(path)
}

func isWSLShell(ctx context.Context, shell string) bool {
	checkWslCmd := exec.CommandContext(ctx, shell, "-c", "uname -r")
	wslOut := &strings.Builder{}
	checkWslCmd.Stdout = wslOut
	err := checkWslCmd.Run()
	if err != nil {
		return false
	}
	return strings.Contains(wslOut.String(), "WSL")
}

func prepareShellContext(a *Action, shell string) (*shellContext, error) {
	rt := a.RuntimeDef()
	isWsl := isWSLShell(context.Background(), shell)
	var convert func(string) string
	if isWsl {
		convert = normalizeContainerMountPath
	} else {
		convert = launchr.ConvertWindowsPath
	}

	// Filter Windows-style paths from environment variables
	vars := convertWindowsVarsPaths(a.getTemplateVars(), convert)
	env := os.Environ()
	env = append(env, vars.envStrings()...)
	env = append(env, rt.Shell.Env...)
	script := rt.Shell.Script
	if isWsl {
		// Prepend environment variables to script
		script = prependEnvToScript(script, env)
	}
	scriptPath, err := exportScriptToFile(script)
	if err != nil {
		return nil, err
	}

	return &shellContext{
		Shell:  shell,
		Env:    env,
		Script: convert(scriptPath),
	}, nil
}

func filterWindowsEnv(env []string) []string {
	filtered := make([]string, 0, len(env))
	prefix := []string{
		"HOMEDRIVE=",
		"PATHEXT=",
		"UID=",
		"SystemDrive=",
		"Chocolatey",
		"$=$",
		":=;",
	}
	for _, e := range env {
		hasPrefix := slices.ContainsFunc(prefix, func(p string) bool {
			return strings.HasPrefix(e, p)
		})
		if !hasPrefix && !strings.Contains(e, `\`) {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

func convertWindowsVarsPaths(v *actionVars, convert func(string) string) *actionVars {
	c := *v
	c.currentBin = convert(v.currentBin)
	c.actionWD = convert(v.actionWD)
	c.actionDir = convert(v.actionDir)
	c.discoveryDir = convert(v.discoveryDir)
	return &c
}

func isExecutable(path string) error {
	// On Windows, check by file extension
	ext := strings.ToLower(filepath.Ext(path))
	extExec := os.Getenv("PATHEXT")
	if extExec == "" {
		extExec = ".COM;.EXE;.BAT;.CMD;.VBS;.VBE;.JS;.JSE;.WSF;.WSH;.MSC"
	}
	extExec = strings.ToUpper(extExec)
	if slices.Contains(strings.Split(extExec, ";"), strings.ToUpper(ext)) {
		return nil
	}
	return errPathNotExecutable
}

// prependEnvToScript prepends all environment variables to the script file
func prependEnvToScript(script string, env []string) string {
	env = filterWindowsEnv(env)
	envStr := ""
	// Write environment variables first
	for _, envVar := range env {
		envStr += "export " + envVar + "\n"
	}

	envStr += "\n"

	return envStr + script
}
