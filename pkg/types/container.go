// Package types contains launchr common types.
package types

import (
	"io"
	"path/filepath"
	"time"

	"github.com/moby/moby/api/types"
	typescontainer "github.com/moby/moby/api/types/container"
	"gopkg.in/yaml.v3"
)

// ResizeOptions is a struct for terminal resizing.
type ResizeOptions = typescontainer.ResizeOptions

// BuildDefinition stores image build definition.
type BuildDefinition struct {
	Context   string             `yaml:"context"`
	Buildfile string             `yaml:"buildfile"`
	Args      map[string]*string `yaml:"args"`
	Tags      []string           `yaml:"tags"`
}

// ImageBuildInfo preprocesses build info to be ready for a container build.
func (b *BuildDefinition) ImageBuildInfo(name string, cwd string) *BuildDefinition {
	if b == nil {
		return nil
	}
	build := *b
	if !filepath.IsAbs(b.Context) {
		build.Context = filepath.Join(cwd, build.Context)
	}
	if name != "" {
		build.Tags = append(build.Tags, name)
	}
	return &build
}

type yamlBuildOptions BuildDefinition

// UnmarshalYAML implements yaml.Unmarshaler to parse build options from a string or a struct.
func (b *BuildDefinition) UnmarshalYAML(n *yaml.Node) (err error) {
	if n.Kind == yaml.ScalarNode {
		var s string
		err = n.Decode(&s)
		*b = BuildDefinition{Context: s}
		return err
	}
	var s yamlBuildOptions
	err = n.Decode(&s)
	if err != nil {
		return err
	}
	*b = BuildDefinition(s)
	return err
}

// ImageOptions stores options for creating/pulling an image.
type ImageOptions struct {
	Name  string
	Build *BuildDefinition
}

// ImageRemoveOptions stores options for removing an image.
type ImageRemoveOptions = types.ImageRemoveOptions

// ImageStatus defines image status on local machine.
type ImageStatus int64

const (
	ImageExists          ImageStatus = iota // ImageExists - image exists locally.
	ImageUnexpectedError                    // ImageUnexpectedError - image can't be pulled or retrieved.
	ImagePull                               // ImagePull - image is being pulled from the registry.
	ImageBuild                              // ImageBuild - image is being built.
	ImageRemoved                            // ImageRemoved - image was removed
)

// ContainerListOptions stores options to request container list.
type ContainerListOptions struct {
	SearchName string
}

// ContainerListResult defines container list result.
type ContainerListResult struct {
	ID     string
	Names  []string
	Status string
}

// ImageStatusResponse stores response when getting the image.
type ImageStatusResponse struct {
	Status   ImageStatus
	Progress io.ReadCloser
}

// ImageRemoveResponse stores response when removing the image.
type ImageRemoveResponse struct {
	Status ImageStatus
}

// ContainerPathStat is a type alias for container path stat result.
type ContainerPathStat = types.ContainerPathStat

// CopyToContainerOptions is a type alias for container copy to container options.
type CopyToContainerOptions = types.CopyToContainerOptions

// NetworkMode is a type alias for container Network mode.
type NetworkMode = typescontainer.NetworkMode

const (
	NetworkModeHost NetworkMode = "host" // NetworkModeHost for host network.
)

// ContainerCreateOptions stores options for creating a new container.
type ContainerCreateOptions struct {
	Hostname      string
	ContainerName string
	Image         string
	Cmd           []string
	WorkingDir    string
	Binds         []string
	Volumes       map[string]struct{}
	NetworkMode   NetworkMode
	ExtraHosts    []string
	AutoRemove    bool
	OpenStdin     bool
	StdinOnce     bool
	AttachStdin   bool
	AttachStdout  bool
	AttachStderr  bool
	Tty           bool
	Env           []string
	User          string
}

// ContainerStartOptions stores options for starting a container.
type ContainerStartOptions struct {
}

// ContainerWaitOptions stores options for waiting while container works.
type ContainerWaitOptions struct {
	Condition WaitCondition
}

// WaitCondition is a type for available wait conditions.
type WaitCondition = typescontainer.WaitCondition

const (
	WaitConditionNotRunning WaitCondition = typescontainer.WaitConditionNotRunning // WaitConditionNotRunning when container exits when running.
	WaitConditionNextExit   WaitCondition = typescontainer.WaitConditionNextExit   // WaitConditionNextExit when container exits after next start.
	WaitConditionRemoved    WaitCondition = typescontainer.WaitConditionRemoved    // WaitConditionRemoved when container is removed.
)

// ContainerWaitResponse stores response given by wait result.
type ContainerWaitResponse struct {
	StatusCode int
	Error      error
}

// ContainerAttachOptions stores options for attaching to a running container.
type ContainerAttachOptions = typescontainer.AttachOptions

// ContainerStopOptions stores options to stop a container.
type ContainerStopOptions struct {
	Timeout *time.Duration
}

// ContainerRemoveOptions stores options to remove a container.
type ContainerRemoveOptions = typescontainer.RemoveOptions
