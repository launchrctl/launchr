// Package driver hold implementation for action drivers.
//
//go:generate mockgen -destination=mocks/driver.go -package=mocks . ContainerRunner
package driver

import (
	"context"
	"io"
	"time"

	"github.com/launchrctl/launchr/core/cli"
)

// ContainerRunner defines common interface for container environments.
type ContainerRunner interface {
	ImageEnsure(ctx context.Context, opts ImageOptions) (*ImageStatusResponse, error)
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

// ResizeOptions is a struct for terminal resizing.
type ResizeOptions struct {
	Height uint
	Width  uint
}

// ImageOptions stores options for creating/pulling an image.
type ImageOptions struct {
	Name  string
	Build *cli.BuildDefinition
}

// ImageStatus defines image status on local machine.
type ImageStatus int64

const (
	ImageExists          ImageStatus = iota // ImageExists - image exists locally.
	ImageUnexpectedError                    // ImageUnexpectedError - image can't be pulled or retrieved.
	ImagePull                               // ImagePull - image is being pulled from the registry.
	ImageBuild                              // ImageBuild - image is being built.
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

// ContainerCreateOptions stores options for creating a new container.
type ContainerCreateOptions struct {
	ContainerName string
	Image         string
	Cmd           []string
	WorkingDir    string
	Mounts        map[string]string
	ExtraHosts    []string
	AutoRemove    bool
	OpenStdin     bool
	StdinOnce     bool
	AttachStdin   bool
	AttachStdout  bool
	AttachStderr  bool
	Tty           bool
	Env           []string
}

// ContainerStartOptions stores options for starting a container.
type ContainerStartOptions struct {
}

// ContainerWaitOptions stores options for waiting while container works.
type ContainerWaitOptions struct {
	Condition WaitCondition
}

// WaitCondition is a type for available wait conditions.
type WaitCondition string

const (
	WaitConditionNotRunning WaitCondition = "not-running" // WaitConditionNotRunning when container exits when running.
	WaitConditionNextExit   WaitCondition = "next-exit"   // WaitConditionNextExit when container exits after next start.
	WaitConditionRemoved    WaitCondition = "removed"     // WaitConditionRemoved when container is removed.
)

// ContainerWaitResponse stores response given by wait result.
type ContainerWaitResponse struct {
	StatusCode int
	Error      error
}

// ContainerAttachOptions stores options for attaching to a running container.
type ContainerAttachOptions struct {
	AttachStdin  bool
	AttachStdout bool
	AttachStderr bool
	Tty          bool
}

// ContainerStopOptions stores options to stop a container.
type ContainerStopOptions struct {
	Timeout *time.Duration
}

// ContainerRemoveOptions stores options to remove a container.
type ContainerRemoveOptions struct {
}
