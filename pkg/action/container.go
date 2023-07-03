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

	"github.com/launchrctl/launchr/pkg/cli"
	"github.com/launchrctl/launchr/pkg/driver"
	"github.com/launchrctl/launchr/pkg/log"
)

const (
	containerHostMount   = "/host"
	containerActionMount = "/action"
)

type containerExec struct {
	driver driver.ContainerRunner
	dtype  driver.Type
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
	return &containerExec{r, t}, nil
}

func (c *containerExec) Exec(ctx context.Context, appCli cli.Cli, cmd *Command) error {
	ctx, cancelFun := context.WithCancel(ctx)
	defer cancelFun()
	a := cmd.Action()
	log.Debug("Starting execution of action %s in %s environment, cmd %v", cmd.CommandName, c.dtype, a.Command)
	// @todo consider reusing the same container and run exec
	name := genContainerName(cmd, nil)
	existing := c.driver.ContainerList(ctx, driver.ContainerListOptions{SearchName: name})
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
	runConfig := &driver.ContainerCreateOptions{
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
	cid, err := c.containerCreate(ctx, appCli, cmd, runConfig)
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
	if err = c.driver.ContainerStart(ctx, cid, driver.ContainerStartOptions{}); err != nil {
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

func (c *containerExec) imageEnsure(ctx context.Context, appCli cli.Cli, cmd *Command) error {
	a := cmd.Action()
	image := a.Image
	var build *cli.BuildDefinition
	cfg := appCli.Config()
	if b := a.BuildDefinition(cmd.Dir()); b != nil {
		build = b
	} else if b = cfg.ImageBuildInfo(image); b != nil {
		build = b
	}
	status, err := c.driver.ImageEnsure(ctx, driver.ImageOptions{
		Name:  image,
		Build: build,
	})
	if err != nil {
		return err
	}
	switch status.Status {
	case driver.ImageExists:
		msg := fmt.Sprintf("Image %s exists locally", image)
		cli.Println(msg)
		log.Info(msg)
	case driver.ImagePull:
		if status.Progress == nil {
			break
		}
		defer func() {
			_ = status.Progress.Close()
		}()
		msg := fmt.Sprintf("Image %s doesn't exist locally, pulling from the registry", image)
		cli.Println(msg)
		log.Info(msg)
		// Output docker status only in Debug.
		log.Debug("Pulling %s progress", image)
		scanner := bufio.NewScanner(status.Progress)
		for scanner.Scan() {
			log.Debug(scanner.Text())
		}
	case driver.ImageBuild:
		if status.Progress == nil {
			break
		}
		defer func() {
			_ = status.Progress.Close()
		}()
		msg := fmt.Sprintf("Image %s doesn't exist locally, building...", image)
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

func (c *containerExec) containerCreate(ctx context.Context, appCli cli.Cli, cmd *Command, config *driver.ContainerCreateOptions) (string, error) {
	if err := c.imageEnsure(ctx, appCli, cmd); err != nil {
		return "", err
	}

	// Create a container
	a := cmd.Action()
	cid, err := c.driver.ContainerCreate(ctx, driver.ContainerCreateOptions{
		ContainerName: config.ContainerName,
		Image:         a.Image,
		Cmd:           a.Command,
		WorkingDir:    containerHostMount,
		Mounts: map[string]string{
			"./":      containerHostMount,
			cmd.Dir(): containerActionMount,
		},
		ExtraHosts:   config.ExtraHosts,
		AutoRemove:   config.AutoRemove,
		OpenStdin:    config.OpenStdin,
		StdinOnce:    config.StdinOnce,
		AttachStdin:  config.AttachStdin,
		AttachStdout: config.AttachStdout,
		AttachStderr: config.AttachStderr,
		Tty:          config.Tty,
		Env:          config.Env,
	})
	if err != nil {
		return "", err
	}

	return cid, nil
}

func (c *containerExec) containerWait(ctx context.Context, cid string, config *driver.ContainerCreateOptions) <-chan int {
	// Wait for the container to stop or catch error.
	waitCond := driver.WaitConditionNextExit
	if config.AutoRemove {
		waitCond = driver.WaitConditionRemoved
	}
	resCh, errCh := c.driver.ContainerWait(ctx, cid, driver.ContainerWaitOptions{Condition: waitCond})
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

func (c *containerExec) attachContainer(ctx context.Context, appCli cli.Cli, cid string, config *driver.ContainerCreateOptions) (io.Closer, <-chan error, error) {
	cio, errAttach := c.driver.ContainerAttach(ctx, cid, driver.ContainerAttachOptions{
		AttachStdin:  config.AttachStdin,
		AttachStdout: config.AttachStdout,
		AttachStderr: config.AttachStderr,
		Tty:          config.Tty,
	})
	if errAttach != nil {
		return nil, nil, errAttach
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- driver.ContainerIOStream(ctx, appCli, cio, config)
	}()
	return cio, errCh, nil
}
