package driver

import (
	"context"
	"errors"
	"io"

	"github.com/launchrctl/launchr/pkg/archive"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
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

func (d *dockerDriver) Info(ctx context.Context) (SystemInfo, error) {
	info, err := d.cli.Info(ctx)
	if err != nil {
		return SystemInfo{}, err
	}
	return SystemInfo{
		ID:              info.ID,
		Name:            info.Name,
		ServerVersion:   info.ServerVersion,
		KernelVersion:   info.KernelVersion,
		OperatingSystem: info.OperatingSystem,
		OSVersion:       info.OSVersion,
		OSType:          info.OSType,
		Architecture:    info.Architecture,
		NCPU:            info.NCPU,
		MemTotal:        info.MemTotal,
		SecurityOptions: info.SecurityOptions,
	}, nil
}

func (d *dockerDriver) IsSELinuxSupported(ctx context.Context) bool {
	info, errInfo := d.cli.Info(ctx)
	if errInfo != nil {
		return false
	}
	for i := 0; i < len(info.SecurityOptions); i++ {
		if info.SecurityOptions[i] == "name=selinux" {
			return true
		}
	}
	return false
}

func (d *dockerDriver) ContainerList(ctx context.Context, opts ContainerListOptions) []ContainerListResult {
	f := filters.NewArgs()
	f.Add("name", opts.SearchName)
	l, err := d.cli.ContainerList(ctx, container.ListOptions{
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

func (d *dockerDriver) ImageEnsure(ctx context.Context, imgOpts ImageOptions) (*ImageStatusResponse, error) {
	// Check if the image already exists.
	insp, _, err := d.cli.ImageInspectWithRaw(ctx, imgOpts.Name)
	if err != nil {
		if !errdefs.IsNotFound(err) {
			return nil, err
		}
	}

	if insp.ID != "" && !imgOpts.ForceRebuild && !imgOpts.NoCache {
		return &ImageStatusResponse{Status: ImageExists}, nil
	}
	// Build the image if it doesn't exist.
	if imgOpts.Build != nil {
		srcInfo, err := archive.CopyInfoSourcePath(imgOpts.Build.Context, false)
		if err != nil {
			return nil, err
		}
		buildContext, errTar := archive.Tar(srcInfo, archive.CopyInfo{}, nil)
		if errTar != nil {
			return nil, errTar
		}
		defer buildContext.Close()
		resp, errBuild := d.cli.ImageBuild(ctx, buildContext, dockertypes.ImageBuildOptions{
			Tags:       []string{imgOpts.Name},
			BuildArgs:  imgOpts.Build.Args,
			Dockerfile: imgOpts.Build.Buildfile,
			NoCache:    imgOpts.NoCache,
		})
		if errBuild != nil {
			return nil, errBuild
		}
		return &ImageStatusResponse{Status: ImageBuild, Progress: resp.Body}, nil
	}
	// Pull the specified image.
	reader, err := d.cli.ImagePull(ctx, imgOpts.Name, image.PullOptions{})
	if err != nil {
		return &ImageStatusResponse{Status: ImageUnexpectedError}, err
	}
	return &ImageStatusResponse{Status: ImagePull, Progress: reader}, nil
}

func (d *dockerDriver) ImageRemove(ctx context.Context, img string, options ImageRemoveOptions) (*ImageRemoveResponse, error) {
	_, err := d.cli.ImageRemove(ctx, img, image.RemoveOptions(options))

	if err != nil {
		return nil, err
	}

	return &ImageRemoveResponse{Status: ImageRemoved}, nil
}

func (d *dockerDriver) CopyToContainer(ctx context.Context, cid string, path string, content io.Reader, opts CopyToContainerOptions) error {
	return d.cli.CopyToContainer(ctx, cid, path, content, container.CopyToContainerOptions(opts))
}

func (d *dockerDriver) CopyFromContainer(ctx context.Context, cid, srcPath string) (io.ReadCloser, ContainerPathStat, error) {
	r, stat, err := d.cli.CopyFromContainer(ctx, cid, srcPath)
	return r, ContainerPathStat(stat), err
}

func (d *dockerDriver) ContainerStatPath(ctx context.Context, cid string, path string) (ContainerPathStat, error) {
	res, err := d.cli.ContainerStatPath(ctx, cid, path)
	return ContainerPathStat(res), err
}

func (d *dockerDriver) ContainerCreate(ctx context.Context, opts ContainerCreateOptions) (string, error) {
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
			Entrypoint:   opts.Entrypoint,
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
	return d.cli.ContainerStart(ctx, cid, container.StartOptions{})
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

func (d *dockerDriver) ContainerAttach(ctx context.Context, containerID string, options ContainerAttachOptions) (*ContainerInOut, error) {
	resp, err := d.cli.ContainerAttach(ctx, containerID, container.AttachOptions(options))
	if err != nil {
		return nil, err
	}

	return &ContainerInOut{In: resp.Conn, Out: resp.Reader}, nil
}

func (d *dockerDriver) ContainerStop(ctx context.Context, cid string) error {
	return d.cli.ContainerStop(ctx, cid, container.StopOptions{})
}

func (d *dockerDriver) ContainerRemove(ctx context.Context, cid string, _ ContainerRemoveOptions) error {
	return d.cli.ContainerRemove(ctx, cid, container.RemoveOptions{})
}

func (d *dockerDriver) ContainerKill(ctx context.Context, containerID, signal string) error {
	return d.cli.ContainerKill(ctx, containerID, signal)
}

func (d *dockerDriver) ContainerResize(ctx context.Context, cid string, opts ResizeOptions) error {
	return d.cli.ContainerResize(ctx, cid, container.ResizeOptions(opts))
}

func (d *dockerDriver) ContainerExecResize(ctx context.Context, cid string, opts ResizeOptions) error {
	return d.cli.ContainerExecResize(ctx, cid, container.ResizeOptions(opts))
}

// Close closes docker cli connection.
func (d *dockerDriver) Close() error {
	return d.cli.Close()
}
