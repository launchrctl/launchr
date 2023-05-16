package core

import (
	"fmt"
	"runtime"
	"time"
)

// AppVersion stores application version.
type AppVersion struct {
	Name      string
	Version   string
	GoVersion string
	BuildDate string
	GitHash   string
	GitBranch string
	OS        string
	Arch      string
	Arm       string
}

func (v *AppVersion) String() string {
	if v.GitHash != "" {
		v.Version += fmt.Sprintf(" (%s)", v.GitHash)
	}
	// If it was not set with ldflags, we need to override at runtime.
	if v.GoVersion == "" {
		v.GoVersion = fmt.Sprintf("%s %s/%s", runtime.Version(), runtime.GOOS, runtime.GOARCH)
		v.BuildDate = time.Now().Format(time.RFC3339)
	}
	return fmt.Sprintf(
		"%s (%s/%s%s) version %s\n"+
			"Built with %s at %s\n",
		v.Name, v.OS, v.Arch, v.Arm, v.Version, v.GoVersion, v.BuildDate,
	)
}
