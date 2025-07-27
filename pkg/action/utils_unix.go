//go:build unix

package action

import (
	"os"
	osuser "os/user"

	"github.com/launchrctl/launchr/internal/launchr"
)

func getCurrentUser() userInfo {
	// If running in a container native environment, run container as a current user.
	curuser := userInfo{}
	u, err := osuser.Current()
	if err == nil {
		curuser.UID = u.Uid
		curuser.GID = u.Gid
	}
	return curuser
}

func normalizeContainerMountPath(path string) string {
	return launchr.MustAbs(path)
}

func prepareShellContext(a *Action, shell string) (*shellContext, error) {
	rt := a.RuntimeDef()
	env := os.Environ()
	env = append(env, a.getTemplateVars().envStrings()...)
	env = append(env, rt.Shell.Env...)
	scriptPath, err := exportScriptToFile(rt.Shell.Script)
	if err != nil {
		return nil, err
	}
	return &shellContext{
		Shell:  shell,
		Env:    env,
		Script: scriptPath,
	}, nil
}

func isExecutable(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.Mode()&0111 == 0 {
		return errPathNotExecutable
	}
	return nil
}
