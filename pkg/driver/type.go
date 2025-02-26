// Package driver hold implementation for container runtimes.
package driver

import (
	"context"
	"io"
	"path/filepath"
	"time"

	typescontainer "github.com/docker/docker/api/types/container"
	typesimage "github.com/docker/docker/api/types/image"
	"gopkg.in/yaml.v3"
)

// ContainerRunner defines common interface for container environments.
type ContainerRunner interface {
	Info(ctx context.Context) (SystemInfo, error)
	CopyToContainer(ctx context.Context, cid string, path string, content io.Reader, opts CopyToContainerOptions) error
	CopyFromContainer(ctx context.Context, cid, srcPath string) (io.ReadCloser, ContainerPathStat, error)
	ContainerStatPath(ctx context.Context, cid string, path string) (ContainerPathStat, error)
	ContainerList(ctx context.Context, opts ContainerListOptions) []ContainerListResult
	ContainerCreate(ctx context.Context, opts ContainerCreateOptions) (string, error)
	ContainerStart(ctx context.Context, cid string, opts ContainerStartOptions) error
	ContainerWait(ctx context.Context, cid string, opts ContainerWaitOptions) (<-chan ContainerWaitResponse, <-chan error)
	ContainerAttach(ctx context.Context, cid string, opts ContainerAttachOptions) (*ContainerInOut, error)
	ContainerStop(ctx context.Context, cid string) error
	ContainerKill(ctx context.Context, cid, signal string) error
	ContainerRemove(ctx context.Context, cid string, opts ContainerRemoveOptions) error
	ContainerResize(ctx context.Context, cid string, opts ResizeOptions) error
	ContainerExecResize(ctx context.Context, cid string, opts ResizeOptions) error
	Close() error
}

// ContainerImageBuilder is an interface for container runtime to build images.
type ContainerImageBuilder interface {
	ContainerRunner
	ImageEnsure(ctx context.Context, opts ImageOptions) (*ImageStatusResponse, error)
	ImageRemove(ctx context.Context, image string, opts ImageRemoveOptions) (*ImageRemoveResponse, error)
}

// ContainerRunnerSELinux defines a container runner with SELinux support.
type ContainerRunnerSELinux interface {
	IsSELinuxSupported(ctx context.Context) bool
}

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

// UnmarshalYAML implements [yaml.Unmarshaler] to parse build options from a string or a struct.
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
	Name         string
	Build        *BuildDefinition
	NoCache      bool
	ForceRebuild bool
}

// ImageRemoveOptions stores options for removing an image.
type ImageRemoveOptions = typesimage.RemoveOptions

// ImageStatus defines image status on local machine.
type ImageStatus int64

// Image statuses.
const (
	ImageExists          ImageStatus = iota // ImageExists - image exists locally.
	ImageUnexpectedError                    // ImageUnexpectedError - image can't be pulled or retrieved.
	ImagePull                               // ImagePull - image is being pulled from the registry.
	ImageBuild                              // ImageBuild - image is being built.
	ImageRemoved                            // ImageRemoved - image was removed
)

// SystemInfo holds information about the container runner environment.
type SystemInfo struct {
	ID              string
	Name            string
	ServerVersion   string
	KernelVersion   string
	OperatingSystem string
	OSVersion       string
	OSType          string
	Architecture    string
	NCPU            int
	MemTotal        int64
	SecurityOptions []string
}

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
type ContainerPathStat = typescontainer.PathStat

// CopyToContainerOptions is a type alias for container copy to container options.
type CopyToContainerOptions = typescontainer.CopyToContainerOptions

// NetworkMode is a type alias for container Network mode.
type NetworkMode = typescontainer.NetworkMode

// Network modes.
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
	Entrypoint    []string
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

// Container wait conditions.
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
