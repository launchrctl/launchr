package action

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/moby/moby/pkg/namesgenerator"
	"github.com/moby/sys/signal"
	"github.com/moby/term"

	"github.com/launchrctl/launchr/internal/launchr/config"
	"github.com/launchrctl/launchr/pkg/cli"
	"github.com/launchrctl/launchr/pkg/driver"
	"github.com/launchrctl/launchr/pkg/log"
	"github.com/launchrctl/launchr/pkg/types"
)

const (
	containerHostMount   = "/host"
	containerActionMount = "/action"
)

type containerExec struct {
	driver driver.ContainerRunner
	dtype  driver.Type
	cfg    config.GlobalConfig
}

// NewDockerExecutor creates a new action executor in Docker environment.
func NewDockerExecutor() (Executor, error) {
	return NewContainerExecutor(driver.Docker)
}

// NewContainerExecutor creates a new action executor in container environment.
func NewContainerExecutor(t driver.Type) (Executor, error) {
	r, err := driver.New(t)
	if err != nil {
		return nil, err
	}
	return &containerExec{driver: r, dtype: t}, nil
}

// SetGlobalConfig implements cli.GlobalConfigAware interface.
func (c *containerExec) SetGlobalConfig(cfg config.GlobalConfig) {
	c.cfg = cfg
}

func (c *containerExec) Exec(ctx context.Context, appCli cli.Streams, cmd *Command) error {
	ctx, cancelFun := context.WithCancel(ctx)
	defer cancelFun()
	a := cmd.Action()
	log.Debug("Starting execution of action %s in %s environment, cmd %v", cmd.CommandName, c.dtype, a.Command)
	// @todo consider reusing the same container and run exec
	name := genContainerName(cmd, nil)
	existing := c.driver.ContainerList(ctx, types.ContainerListOptions{SearchName: name})
	// Collect a set of existing names to build the name.
	exMap := make(map[string]struct{}, len(existing))
	for _, e := range existing {
		for _, n := range e.Names {
			exMap[strings.Trim(n, "/")] = struct{}{}
		}
	}
	// Regenerate the name with a suffix.
	if len(exMap) > 0 {
		name = genContainerName(cmd, exMap)
	}

	// Create container.
	runConfig := &types.ContainerCreateOptions{
		ContainerName: name,
		ExtraHosts:    a.ExtraHosts,
		AutoRemove:    true,
		OpenStdin:     true,
		StdinOnce:     true,
		AttachStdin:   true,
		AttachStdout:  true,
		AttachStderr:  true,
		Tty:           appCli.In().IsTerminal(),
		Env:           a.Env,
	}
	cid, err := c.containerCreate(ctx, cmd, runConfig)
	if err != nil {
		return err
	}
	if cid == "" {
		return errors.New("error on creating a container")
	}

	// Check if TTY was requested, but not supported.
	if ttyErr := appCli.In().CheckTty(runConfig.AttachStdin, runConfig.Tty); ttyErr != nil {
		return ttyErr
	}

	if !runConfig.Tty {
		sigc := notifyAllSignals()
		go ForwardAllSignals(ctx, c.driver, cid, sigc)
		defer signal.StopCatch(sigc)
	}

	// Attach streams to the terminal.
	cio, errCh, err := c.attachContainer(ctx, appCli, cid, runConfig)
	if err != nil {
		return err
	}
	defer func() {
		_ = cio.Close()
	}()
	statusCh := c.containerWait(ctx, cid, runConfig)

	// Start the container
	if err = c.driver.ContainerStart(ctx, cid, types.ContainerStartOptions{}); err != nil {
		cancelFun()
		<-errCh
		if runConfig.AutoRemove {
			<-statusCh
		}
		return err
	}

	// Resize TTY on window resize.
	if runConfig.Tty {
		if err := driver.MonitorTtySize(ctx, c.driver, appCli, cid, false); err != nil {
			log.Err("Error monitoring TTY size:", err)
		}
	}

	if errCh != nil {
		if err := <-errCh; err != nil {
			if _, ok := err.(term.EscapeError); ok {
				// The user entered the detach escape sequence.
				return nil
			}

			log.Debug("Error hijack: %s", err)
			return err
		}
	}

	status := <-statusCh
	if status != 0 {
		return errors.New("error on run")
	}
	return nil
}

func genContainerName(cmd *Command, existing map[string]struct{}) string {
	// Replace command name "-", ":", and "." to "_".
	replace := "-:."
	od := make([]string, len(replace)*2)
	for i, c := range replace {
		od[2*i], od[2*i+1] = string(c), "_"
	}
	rpl := strings.NewReplacer(od...)
	base := "launchr_" + rpl.Replace(cmd.CommandName)
	name := base
	if len(existing) > 0 {
		_, ok := existing[name]
		// Set suffix if container already exists.
		for ; ok; _, ok = existing[name] {
			name = base + "_" + namesgenerator.GetRandomName(0)
		}
	}
	return name
}

func (c *containerExec) Close() error {
	return c.driver.Close()
}

func (c *containerExec) imageEnsure(ctx context.Context, cmd *Command) error {
	a := cmd.Action()
	image := a.Image
	var build *types.BuildDefinition
	if b := a.BuildDefinition(cmd.Dir()); b != nil {
		build = b
	} else if b = GlobalConfigImage(c.cfg, image); b != nil {
		build = b
	}
	status, err := c.driver.ImageEnsure(ctx, types.ImageOptions{
		Name:  image,
		Build: build,
	})
	if err != nil {
		return err
	}
	switch status.Status {
	case types.ImageExists:
		msg := fmt.Sprintf("Image %q exists locally", image)
		cli.Println(msg)
		log.Info(msg)
	case types.ImagePull:
		if status.Progress == nil {
			break
		}
		defer func() {
			_ = status.Progress.Close()
		}()
		msg := fmt.Sprintf("Image %q doesn't exist locally, pulling from the registry", image)
		cli.Println(msg)
		log.Info(msg)
		// Output docker status only in Debug.
		log.Debug("Pulling %s progress", image)
		scanner := bufio.NewScanner(status.Progress)
		for scanner.Scan() {
			log.Debug(scanner.Text())
		}
	case types.ImageBuild:
		if status.Progress == nil {
			break
		}
		defer func() {
			_ = status.Progress.Close()
		}()
		msg := fmt.Sprintf("Image %q doesn't exist locally, building...", image)
		cli.Println(msg)
		log.Info(msg)
		// Output docker status only in Debug.
		log.Debug("Building %s progress", image)
		scanner := bufio.NewScanner(status.Progress)
		for scanner.Scan() {
			log.Debug(scanner.Text())
		}
	}

	return nil
}

func (c *containerExec) containerCreate(ctx context.Context, cmd *Command, opts *types.ContainerCreateOptions) (string, error) {
	if err := c.imageEnsure(ctx, cmd); err != nil {
		return "", err
	}

	// Create a container
	a := cmd.Action()
	cid, err := c.driver.ContainerCreate(ctx, types.ContainerCreateOptions{
		ContainerName: opts.ContainerName,
		Image:         a.Image,
		Cmd:           a.Command,
		WorkingDir:    containerHostMount,
		Mounts: map[string]string{
			"./":      containerHostMount,
			cmd.Dir(): containerActionMount,
		},
		ExtraHosts:   opts.ExtraHosts,
		AutoRemove:   opts.AutoRemove,
		OpenStdin:    opts.OpenStdin,
		StdinOnce:    opts.StdinOnce,
		AttachStdin:  opts.AttachStdin,
		AttachStdout: opts.AttachStdout,
		AttachStderr: opts.AttachStderr,
		Tty:          opts.Tty,
		Env:          opts.Env,
	})
	if err != nil {
		return "", err
	}

	return cid, nil
}

func (c *containerExec) containerWait(ctx context.Context, cid string, opts *types.ContainerCreateOptions) <-chan int {
	// Wait for the container to stop or catch error.
	waitCond := types.WaitConditionNextExit
	if opts.AutoRemove {
		waitCond = types.WaitConditionRemoved
	}
	resCh, errCh := c.driver.ContainerWait(ctx, cid, types.ContainerWaitOptions{Condition: waitCond})
	statusC := make(chan int)
	go func() {
		select {
		case err := <-errCh:
			log.Err("error waiting for container: %v", err)
			statusC <- 125
		case res := <-resCh:
			if res.Error != nil {
				log.Err("error waiting for container: %v", res.Error)
				statusC <- 125
			} else {
				statusC <- res.StatusCode
			}
		case <-ctx.Done():
			log.Info("stopping waiting for container on context finish")
			statusC <- 125
		}
	}()

	return statusC
}

func (c *containerExec) attachContainer(ctx context.Context, appCli cli.Streams, cid string, opts *types.ContainerCreateOptions) (io.Closer, <-chan error, error) {
	cio, errAttach := c.driver.ContainerAttach(ctx, cid, types.ContainerAttachOptions{
		AttachStdin:  opts.AttachStdin,
		AttachStdout: opts.AttachStdout,
		AttachStderr: opts.AttachStderr,
		Tty:          opts.Tty,
	})
	if errAttach != nil {
		return nil, nil, errAttach
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- driver.ContainerIOStream(ctx, appCli, cio, opts)
	}()
	return cio, errCh, nil
}

// GlobalConfigImagesKey is a field name in global config file.
const GlobalConfigImagesKey = "images"

// GlobalConfigImages is a container to parse global config with yaml.
type GlobalConfigImages map[string]*types.BuildDefinition

// GlobalConfigImage extends GlobalConfig functionality and parses images definition.
func GlobalConfigImage(cfg config.GlobalConfig, image string) *types.BuildDefinition {
	var images GlobalConfigImages
	err := cfg.Get(GlobalConfigImagesKey, &images)
	if err != nil {
		log.Warn("global configuration field %q is malformed", GlobalConfigImagesKey)
		return nil
	}
	if b, ok := images[image]; ok {
		return b.BuildImageInfo(image, cfg.DirPath())
	}
	for _, b := range images {
		for _, t := range b.Tags {
			if t == image {
				return b.BuildImageInfo(image, cfg.DirPath())
			}
		}
	}
	return nil
}
