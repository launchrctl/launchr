// Package driver hold implementation for action drivers.
//
//go:generate go run go.uber.org/mock/mockgen@latest -destination=mocks/driver.go -package=mocks . ContainerRunner
package driver

import (
	"context"
	"io"

	"github.com/launchrctl/launchr/pkg/types"
)

// ContainerRunner defines common interface for container environments.
type ContainerRunner interface {
	Info(ctx context.Context) (types.SystemInfo, error)
	ImageEnsure(ctx context.Context, opts types.ImageOptions) (*types.ImageStatusResponse, error)
	ImageRemove(ctx context.Context, image string, opts types.ImageRemoveOptions) (*types.ImageRemoveResponse, error)
	CopyToContainer(ctx context.Context, cid string, path string, content io.Reader, opts types.CopyToContainerOptions) error
	CopyFromContainer(ctx context.Context, cid, srcPath string) (io.ReadCloser, types.ContainerPathStat, error)
	ContainerStatPath(ctx context.Context, cid string, path string) (types.ContainerPathStat, error)
	ContainerList(ctx context.Context, opts types.ContainerListOptions) []types.ContainerListResult
	ContainerCreate(ctx context.Context, opts types.ContainerCreateOptions) (string, error)
	ContainerStart(ctx context.Context, cid string, opts types.ContainerStartOptions) error
	ContainerWait(ctx context.Context, cid string, opts types.ContainerWaitOptions) (<-chan types.ContainerWaitResponse, <-chan error)
	ContainerAttach(ctx context.Context, cid string, opts types.ContainerAttachOptions) (*ContainerInOut, error)
	ContainerStop(ctx context.Context, cid string) error
	ContainerKill(ctx context.Context, cid, signal string) error
	ContainerRemove(ctx context.Context, cid string, opts types.ContainerRemoveOptions) error
	ContainerResize(ctx context.Context, cid string, opts types.ResizeOptions) error
	ContainerExecResize(ctx context.Context, cid string, opts types.ResizeOptions) error
	Close() error
}

// ContainerRunnerSELinux defines a container runner with SELinux support.
type ContainerRunnerSELinux interface {
	IsSELinuxSupported(ctx context.Context) bool
}
