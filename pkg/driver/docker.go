package driver

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/launchrctl/launchr/internal/launchr"
	"github.com/launchrctl/launchr/pkg/archive"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/pkg/stdcopy"
)

const dockerNetworkModeHost = "host"

// ContainerWaitResponse stores response given by wait result.
type dockerWaitResponse struct {
	StatusCode int
	Error      error
}

type dockerRuntime struct {
	cli  client.APIClient
	info SystemInfo
}

// NewDockerRuntime creates a docker runtime.
func NewDockerRuntime() (ContainerRunner, error) {
	// @todo it doesn't work with Colima or with non-default context.
	c, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())

	if err != nil {
		return nil, err
	}
	return &dockerRuntime{cli: c}, nil
}

func (d *dockerRuntime) Info(ctx context.Context) (SystemInfo, error) {
	if d.info.ID != "" {
		return d.info, nil
	}
	info, err := d.cli.Info(ctx)
	if err != nil {
		return SystemInfo{}, err
	}
	d.info = SystemInfo{
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
		// @todo consider remote environments where we can't directly bind local dirs.
		Remote: false,
	}
	return d.info, nil
}

func (d *dockerRuntime) IsSELinuxSupported(ctx context.Context) bool {
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

func (d *dockerRuntime) ContainerList(ctx context.Context, opts ContainerListOptions) []ContainerListResult {
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

func (d *dockerRuntime) ImageEnsure(ctx context.Context, imgOpts ImageOptions) (*ImageStatusResponse, error) {
	// Check if the image already exists.
	insp, err := d.cli.ImageInspect(ctx, imgOpts.Name)
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

func (d *dockerRuntime) ImageRemove(ctx context.Context, img string, options ImageRemoveOptions) (*ImageRemoveResponse, error) {
	_, err := d.cli.ImageRemove(ctx, img, image.RemoveOptions{
		Force: options.Force,
	})

	if err != nil {
		return nil, err
	}

	return &ImageRemoveResponse{Status: ImageRemoved}, nil
}

func (d *dockerRuntime) CopyToContainer(ctx context.Context, cid string, path string, content io.Reader, opts CopyToContainerOptions) error {
	return d.cli.CopyToContainer(ctx, cid, path, content, container.CopyToContainerOptions(opts))
}

func (d *dockerRuntime) CopyFromContainer(ctx context.Context, cid, srcPath string) (io.ReadCloser, ContainerPathStat, error) {
	r, stat, err := d.cli.CopyFromContainer(ctx, cid, srcPath)
	return r, ContainerPathStat(stat), err
}

func (d *dockerRuntime) ContainerStatPath(ctx context.Context, cid string, path string) (ContainerPathStat, error) {
	res, err := d.cli.ContainerStatPath(ctx, cid, path)
	return ContainerPathStat(res), err
}

func (d *dockerRuntime) ContainerCreate(ctx context.Context, opts ContainerDefinition) (string, error) {
	hostCfg := &container.HostConfig{
		ExtraHosts:  opts.ExtraHosts,
		NetworkMode: dockerNetworkModeHost,
		Binds:       opts.Binds,
	}

	// Prepare volumes.
	volumes := make(map[string]struct{}, len(opts.Volumes))
	for i := 0; i < len(opts.Volumes); i++ {
		volume := ""
		if opts.Volumes[i].Name != "" {
			volume += opts.Volumes[i].Name + ":"
		}
		volume += opts.Volumes[i].MountPath
	}

	resp, err := d.cli.ContainerCreate(
		ctx,
		&container.Config{
			Hostname:     opts.Hostname,
			Image:        opts.Image,
			Cmd:          opts.Command,
			WorkingDir:   opts.WorkingDir,
			OpenStdin:    opts.Streams.Stdin,
			AttachStdin:  opts.Streams.Stdin,
			AttachStdout: opts.Streams.Stdout,
			AttachStderr: opts.Streams.Stderr,
			Tty:          opts.Streams.TTY,
			Env:          opts.Env,
			User:         opts.User,
			Volumes:      volumes,
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

func (d *dockerRuntime) ContainerStart(ctx context.Context, cid string, runConfig ContainerDefinition) (<-chan int, *ContainerInOut, error) {
	// Attach streams to the terminal.
	launchr.Log().Debug("attaching container streams")
	cio, err := d.ContainerAttach(ctx, cid, runConfig.Streams)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to attach to the container: %w", err)
	}
	defer func() {
		if err != nil {
			_ = cio.Close()
		}
	}()

	launchr.Log().Debug("watching run status of container")
	statusCh := d.doContainerWait(ctx, cid)

	err = d.cli.ContainerStart(ctx, cid, container.StartOptions{})
	if err != nil {
		return nil, nil, err
	}

	return statusCh, cio, nil
}

func (d *dockerRuntime) containerWait(ctx context.Context, cid string) (<-chan dockerWaitResponse, <-chan error) {
	statusCh, errCh := d.cli.ContainerWait(ctx, cid, container.WaitConditionNextExit)

	wrappedStCh := make(chan dockerWaitResponse)
	go func() {
		st := <-statusCh
		var err error
		if st.Error != nil {
			err = errors.New(st.Error.Message)
		}
		wrappedStCh <- dockerWaitResponse{
			StatusCode: int(st.StatusCode),
			Error:      err,
		}
	}()

	return wrappedStCh, errCh
}

func (d *dockerRuntime) ContainerAttach(ctx context.Context, cid string, options ContainerStreamsOptions) (*ContainerInOut, error) {
	resp, err := d.cli.ContainerAttach(ctx, cid, container.AttachOptions{
		Stream: true,
		Stdin:  options.Stdin,
		Stdout: options.Stdout,
		Stderr: options.Stderr,
	})
	if err != nil {
		return nil, err
	}

	cio := &ContainerInOut{
		In:   resp.Conn,
		Out:  resp.Reader,
		Opts: options,
	}

	// Resize TTY on window resize.
	if options.TTY {
		cio.TtyMonitor = NewTtySizeMonitor(func(ctx context.Context, ropts terminalSize) error {
			return d.cli.ContainerResize(ctx, cid, container.ResizeOptions(ropts))
		})
	}

	// Only need to demultiplex if we have multiplexed output
	if !options.TTY && resp.Reader != nil && options.Stdout {
		// Create pipes for stdout and stderr
		stdoutReader, stdoutWriter := io.Pipe()
		stderrReader, stderrWriter := io.Pipe()

		// Start demultiplexing in a goroutine
		go func() {
			defer stdoutWriter.Close()
			defer stderrWriter.Close()

			// Demultiplex the output stream into our pipes
			_, err := stdcopy.StdCopy(stdoutWriter, stderrWriter, resp.Reader)
			if err != nil {
				// If an error occurs during demultiplexing, write it to the stderr pipe
				launchr.Log().Error("\"error demultiplexing container output", "error", err)
			}
		}()
		cio.Out = stdoutReader
		cio.Err = stderrReader

		// Return the ContainerInOut with demultiplexed streams
		return cio, nil
	}

	// If no demultiplexing is needed, return the raw connection
	return cio, nil

}

func (d *dockerRuntime) ContainerStop(ctx context.Context, cid string, opts ContainerStopOptions) error {
	var timeout *int
	if opts.Timeout != nil {
		t := int(opts.Timeout.Seconds())
		timeout = &t
	}
	return d.cli.ContainerStop(ctx, cid, container.StopOptions{
		Timeout: timeout,
	})
}

func (d *dockerRuntime) ContainerRemove(ctx context.Context, cid string) error {
	return d.cli.ContainerRemove(ctx, cid, container.RemoveOptions{
		RemoveVolumes: true,
	})
}

func (d *dockerRuntime) ContainerKill(ctx context.Context, containerID, signal string) error {
	return d.cli.ContainerKill(ctx, containerID, signal)
}

// Close closes docker cli connection.
func (d *dockerRuntime) Close() error {
	return d.cli.Close()
}

func (d *dockerRuntime) doContainerWait(ctx context.Context, cid string) <-chan int {
	// Wait for the container to stop or catch error.
	resCh, errCh := d.containerWait(ctx, cid)
	statusC := make(chan int)
	go func() {
		select {
		case err := <-errCh:
			launchr.Log().Error("error waiting for container", "error", err)
			statusC <- 125
		case res := <-resCh:
			if res.Error != nil {
				launchr.Log().Error("error in container run", "error", res.Error)
				statusC <- 125
			} else {
				launchr.Log().Debug("received run status code", "exit_code", res.StatusCode)
				statusC <- res.StatusCode
			}
		case <-ctx.Done():
			launchr.Log().Debug("stop waiting for container on context finish")
			statusC <- 125
		}
	}()

	return statusC
}
