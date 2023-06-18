package launchr

import (
	"fmt"
	"runtime"
	"time"
)

var (
	Name          = "launchr" // Name - version info
	Version       = "dev"     // Version - version info
	GoVersion     string      // GoVersion - version info
	BuildDate     string      // BuildDate - version info
	GitHash       string      // GitHash - version info
	GitBranch     string      // GitBranch - version info
	CustomVersion string      // CustomVersion - custom version text
)

// AppVersion stores application version.
type AppVersion struct {
	Name          string
	Version       string
	GoVersion     string
	BuildDate     string
	GitHash       string
	GitBranch     string
	OS            string
	Arch          string
	Arm           string
	CustomVersion string
}

// SetCustomVersion stores custom version string for output, for example on custom build.
func SetCustomVersion(v string) {
	CustomVersion = v
}

var appVersion *AppVersion

// GetVersion provides app version info.
func GetVersion() *AppVersion {
	if appVersion == nil {
		appVersion = &AppVersion{
			Name:          Name,
			Version:       Version,
			GoVersion:     GoVersion,
			BuildDate:     BuildDate,
			GitHash:       GitHash,
			GitBranch:     GitBranch,
			OS:            runtime.GOOS,
			Arch:          runtime.GOARCH,
			CustomVersion: CustomVersion,
		}

	}
	return appVersion
}

func (v *AppVersion) String() string {
	if len(CustomVersion) > 0 {
		return CustomVersion
	}
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
