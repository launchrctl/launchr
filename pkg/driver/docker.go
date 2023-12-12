package driver

import (
	"context"
	"errors"
	"io"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/moby/moby/client"
	"github.com/moby/moby/errdefs"
	"github.com/moby/moby/pkg/archive"

	"github.com/launchrctl/launchr/pkg/types"
)

type dockerDriver struct {
	cli client.APIClient
}

// NewDockerDriver creates a docker driver.
func NewDockerDriver() (ContainerRunner, error) {
	// @todo it doesn't work with Colima or with non-default context.
	c, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())

	if err != nil {
		return nil, err
	}
	return &dockerDriver{cli: c}, nil
}

func (d *dockerDriver) ContainerList(ctx context.Context, opts types.ContainerListOptions) []types.ContainerListResult {
	f := filters.NewArgs()
	f.Add("name", opts.SearchName)
	l, err := d.cli.ContainerList(ctx, dockertypes.ContainerListOptions{
		Filters: f,
		All:     true,
	})
	if err != nil {
		return nil
	}
	lp := make([]types.ContainerListResult, len(l))
	for i, c := range l {
		lp[i] = types.ContainerListResult{
			ID:     c.ID,
			Names:  c.Names,
			Status: c.Status,
		}
	}
	return lp
}

func (d *dockerDriver) ImageEnsure(ctx context.Context, image types.ImageOptions) (*types.ImageStatusResponse, error) {
	// Check if the image already exists.
	insp, _, err := d.cli.ImageInspectWithRaw(ctx, image.Name)
	if err != nil {
		if !errdefs.IsNotFound(err) {
			return nil, err
		}
	}
	if insp.ID != "" {
		return &types.ImageStatusResponse{Status: types.ImageExists}, nil
	}
	// Build the image if it doesn't exist.
	if image.Build != nil {
		buildContext, errTar := archive.TarWithOptions(image.Build.Context, &archive.TarOptions{})
		if errTar != nil {
			return nil, errTar
		}
		resp, errBuild := d.cli.ImageBuild(ctx, buildContext, dockertypes.ImageBuildOptions{
			Tags:       []string{image.Name},
			BuildArgs:  image.Build.Args,
			Dockerfile: image.Build.Buildfile,
		})
		if errBuild != nil {
			return nil, errBuild
		}
		return &types.ImageStatusResponse{Status: types.ImageBuild, Progress: resp.Body}, nil
	}
	// Pull the specified image.
	reader, err := d.cli.ImagePull(ctx, image.Name, dockertypes.ImagePullOptions{})
	if err != nil {
		return &types.ImageStatusResponse{Status: types.ImageUnexpectedError}, err
	}
	return &types.ImageStatusResponse{Status: types.ImagePull, Progress: reader}, nil
}

func (d *dockerDriver) ImageRemove(ctx context.Context, image string, options types.ImageRemoveOptions) (*types.ImageRemoveResponse, error) {
	_, err := d.cli.ImageRemove(ctx, image, dockertypes.ImageRemoveOptions(options))

	if err != nil {
		return nil, err
	}

	return &types.ImageRemoveResponse{Status: types.ImageRemoved}, nil
}

func (d *dockerDriver) CopyToContainer(ctx context.Context, cid string, path string, content io.Reader, opts types.CopyToContainerOptions) error {
	return d.cli.CopyToContainer(ctx, cid, path, content, dockertypes.CopyToContainerOptions(opts))
}

func (d *dockerDriver) CopyFromContainer(ctx context.Context, cid, srcPath string) (io.ReadCloser, types.ContainerPathStat, error) {
	r, stat, err := d.cli.CopyFromContainer(ctx, cid, srcPath)
	return r, types.ContainerPathStat(stat), err
}

func (d *dockerDriver) ContainerStatPath(ctx context.Context, cid string, path string) (types.ContainerPathStat, error) {
	res, err := d.cli.ContainerStatPath(ctx, cid, path)
	return types.ContainerPathStat(res), err
}

func (d *dockerDriver) ContainerCreate(ctx context.Context, opts types.ContainerCreateOptions) (string, error) {
	hostCfg := &container.HostConfig{
		AutoRemove:  opts.AutoRemove,
		ExtraHosts:  opts.ExtraHosts,
		NetworkMode: container.NetworkMode(opts.NetworkMode),
		Binds:       opts.Binds,
	}

	resp, err := d.cli.ContainerCreate(
		ctx,
		&container.Config{
			Hostname:     opts.Hostname,
			Image:        opts.Image,
			Cmd:          opts.Cmd,
			WorkingDir:   opts.WorkingDir,
			OpenStdin:    opts.OpenStdin,
			StdinOnce:    opts.StdinOnce,
			AttachStdin:  opts.AttachStdin,
			AttachStdout: opts.AttachStdout,
			AttachStderr: opts.AttachStderr,
			Tty:          opts.Tty,
			Env:          opts.Env,
			User:         opts.User,
			Volumes:      opts.Volumes,
		},
		hostCfg,
		nil, nil, opts.ContainerName,
	)
	if err != nil {
		return "", err
	}

	return resp.ID, nil
}

func (d *dockerDriver) ContainerStart(ctx context.Context, cid string, _ types.ContainerStartOptions) error {
	return d.cli.ContainerStart(ctx, cid, dockertypes.ContainerStartOptions{})
}

func (d *dockerDriver) ContainerWait(ctx context.Context, cid string, opts types.ContainerWaitOptions) (<-chan types.ContainerWaitResponse, <-chan error) {
	statusCh, errCh := d.cli.ContainerWait(ctx, cid, container.WaitCondition(opts.Condition))

	wrappedStCh := make(chan types.ContainerWaitResponse)
	go func() {
		st := <-statusCh
		var err error
		if st.Error != nil {
			err = errors.New(st.Error.Message)
		}
		wrappedStCh <- types.ContainerWaitResponse{
			StatusCode: int(st.StatusCode),
			Error:      err,
		}
	}()

	return wrappedStCh, errCh
}

func (d *dockerDriver) ContainerAttach(ctx context.Context, containerID string, options types.ContainerAttachOptions) (*ContainerInOut, error) {
	resp, err := d.cli.ContainerAttach(ctx, containerID, dockertypes.ContainerAttachOptions(options))
	if err != nil {
		return nil, err
	}

	return &ContainerInOut{In: resp.Conn, Out: resp.Reader}, nil
}

func (d *dockerDriver) ContainerStop(ctx context.Context, cid string) error {
	return d.cli.ContainerStop(ctx, cid, container.StopOptions{})
}

func (d *dockerDriver) ContainerRemove(ctx context.Context, cid string, _ types.ContainerRemoveOptions) error {
	return d.cli.ContainerRemove(ctx, cid, dockertypes.ContainerRemoveOptions{})
}

func (d *dockerDriver) ContainerKill(ctx context.Context, containerID, signal string) error {
	return d.cli.ContainerKill(ctx, containerID, signal)
}

func (d *dockerDriver) ContainerResize(ctx context.Context, cid string, opts types.ResizeOptions) error {
	return d.cli.ContainerResize(ctx, cid, dockertypes.ResizeOptions{
		Height: opts.Height,
		Width:  opts.Width,
	})
}

func (d *dockerDriver) ContainerExecResize(ctx context.Context, cid string, opts types.ResizeOptions) error {
	return d.cli.ContainerExecResize(ctx, cid, dockertypes.ResizeOptions{
		Height: opts.Height,
		Width:  opts.Width,
	})
}

// Close closes docker cli connection.
func (d *dockerDriver) Close() error {
	return d.cli.Close()
}
