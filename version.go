package launchr

import (
	"fmt"
	"runtime"
)

var (
	Name          = "launchr" // Name - version info
	Version       = "dev"     // Version - version info
	CustomVersion string      // CustomVersion - custom version text
)

// AppVersion stores application version.
type AppVersion struct {
	Name          string
	Version       string
	OS            string
	Arch          string
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
	return fmt.Sprintf("%s version %s %s/%s\n", v.Name, v.Version, v.OS, v.Arch)
}
