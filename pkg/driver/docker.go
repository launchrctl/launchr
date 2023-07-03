package driver

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	"github.com/moby/moby/client"
	"github.com/moby/moby/errdefs"
	"github.com/moby/moby/pkg/archive"
)

type dockerDriver struct {
	cli client.APIClient
}

// NewDockerDriver creates a docker driver.
func NewDockerDriver() (ContainerRunner, error) {
	c, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())

	if err != nil {
		return nil, err
	}
	return &dockerDriver{cli: c}, nil
}

func (d *dockerDriver) ContainerList(ctx context.Context, opts ContainerListOptions) []ContainerListResult {
	f := filters.NewArgs()
	f.Add("name", opts.SearchName)
	l, err := d.cli.ContainerList(ctx, types.ContainerListOptions{
		Filters: f,
		All:     true,
	})
	if err != nil {
		return nil
	}
	lp := make([]ContainerListResult, len(l))
	for i, c := range l {
		lp[i] = ContainerListResult{
			ID:     c.ID,
			Names:  c.Names,
			Status: c.Status,
		}
	}
	return lp
}

func (d *dockerDriver) ImageEnsure(ctx context.Context, image ImageOptions) (*ImageStatusResponse, error) {
	// Check if the image already exists.
	insp, _, err := d.cli.ImageInspectWithRaw(ctx, image.Name)
	if err != nil {
		if !errdefs.IsNotFound(err) {
			return nil, err
		}
	}
	if insp.ID != "" {
		return &ImageStatusResponse{Status: ImageExists}, nil
	}
	// Build the image if it doesn't exist.
	if image.Build != nil {
		buildContext, errTar := archive.TarWithOptions(image.Build.Context, &archive.TarOptions{})
		if errTar != nil {
			return nil, errTar
		}
		resp, errBuild := d.cli.ImageBuild(ctx, buildContext, types.ImageBuildOptions{
			Tags:       []string{image.Name},
			BuildArgs:  image.Build.Args,
			Dockerfile: image.Build.Buildfile,
		})
		if errBuild != nil {
			return nil, errBuild
		}
		return &ImageStatusResponse{Status: ImageBuild, Progress: resp.Body}, nil
	}
	// Pull the specified image.
	reader, err := d.cli.ImagePull(ctx, image.Name, types.ImagePullOptions{})
	if err != nil {
		return &ImageStatusResponse{Status: ImageUnexpectedError}, err
	}
	return &ImageStatusResponse{Status: ImagePull, Progress: reader}, nil
}

func (d *dockerDriver) ContainerCreate(ctx context.Context, opts ContainerCreateOptions) (string, error) {
	hostCfg := &container.HostConfig{
		AutoRemove: opts.AutoRemove,
		ExtraHosts: opts.ExtraHosts,
	}
	if len(opts.Mounts) > 0 {
		mounts := make([]mount.Mount, 0, len(opts.Mounts))
		for s, t := range opts.Mounts {
			abs, err := filepath.Abs(s)
			if err != nil {
				return "", err
			}
			mounts = append(mounts, mount.Mount{
				Type:   mount.TypeBind,
				Source: abs,
				Target: t,
			})
		}
		hostCfg.Mounts = mounts
	}

	resp, err := d.cli.ContainerCreate(
		ctx,
		&container.Config{
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
		},
		hostCfg,
		nil, nil, opts.ContainerName,
	)
	if err != nil {
		return "", err
	}

	return resp.ID, nil
}

func (d *dockerDriver) ContainerStart(ctx context.Context, cid string, _ ContainerStartOptions) error {
	return d.cli.ContainerStart(ctx, cid, types.ContainerStartOptions{})
}

func (d *dockerDriver) ContainerWait(ctx context.Context, cid string, opts ContainerWaitOptions) (<-chan ContainerWaitResponse, <-chan error) {
	statusCh, errCh := d.cli.ContainerWait(ctx, cid, container.WaitCondition(opts.Condition))

	wrappedStCh := make(chan ContainerWaitResponse)
	go func() {
		st := <-statusCh
		var err error
		if st.Error != nil {
			err = errors.New(st.Error.Message)
		}
		wrappedStCh <- ContainerWaitResponse{
			StatusCode: int(st.StatusCode),
			Error:      err,
		}
	}()

	return wrappedStCh, errCh
}

func (d *dockerDriver) ContainerAttach(ctx context.Context, containerID string, config ContainerAttachOptions) (*ContainerInOut, error) {
	options := types.ContainerAttachOptions{
		Stream: true,
		Stdin:  config.AttachStdin,
		Stdout: config.AttachStdout,
		Stderr: config.AttachStderr,
	}

	resp, err := d.cli.ContainerAttach(ctx, containerID, options)
	if err != nil {
		return nil, err
	}

	return &ContainerInOut{In: resp.Conn, Out: resp.Reader}, nil
}

func (d *dockerDriver) ContainerStop(ctx context.Context, cid string) error {
	return d.cli.ContainerStop(ctx, cid, container.StopOptions{})
}

func (d *dockerDriver) ContainerRemove(ctx context.Context, cid string, _ ContainerRemoveOptions) error {
	return d.cli.ContainerRemove(ctx, cid, types.ContainerRemoveOptions{})
}

func (d *dockerDriver) ContainerKill(ctx context.Context, containerID, signal string) error {
	return d.cli.ContainerKill(ctx, containerID, signal)
}

func (d *dockerDriver) ContainerResize(ctx context.Context, cid string, opts ResizeOptions) error {
	return d.cli.ContainerResize(ctx, cid, types.ResizeOptions{
		Height: opts.Height,
		Width:  opts.Width,
	})
}

func (d *dockerDriver) ContainerExecResize(ctx context.Context, cid string, opts ResizeOptions) error {
	return d.cli.ContainerExecResize(ctx, cid, types.ResizeOptions{
		Height: opts.Height,
		Width:  opts.Width,
	})
}

// Close closes docker cli connection.
func (d *dockerDriver) Close() error {
	return d.cli.Close()
}
