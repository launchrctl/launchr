package action

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	osuser "os/user"
	"runtime"
	"strings"

	"github.com/moby/moby/pkg/namesgenerator"
	"github.com/moby/sys/signal"
	"github.com/moby/term"

	"github.com/launchrctl/launchr/pkg/cli"
	"github.com/launchrctl/launchr/pkg/driver"
	"github.com/launchrctl/launchr/pkg/log"
	"github.com/launchrctl/launchr/pkg/types"
)

const (
	containerHostMount   = "/host"
	containerActionMount = "/action"
)

type containerEnv struct {
	driver driver.ContainerRunner
	imgres ChainImageBuildResolver
	dtype  driver.Type
	prefix string
}

// NewDockerEnvironment creates a new action Docker environment.
func NewDockerEnvironment() RunEnvironment {
	return NewContainerEnvironment(driver.Docker)
}

// NewContainerEnvironment creates a new action container run environment.
func NewContainerEnvironment(t driver.Type) RunEnvironment {
	return &containerEnv{dtype: t, prefix: "launchr_"}
}

func (c *containerEnv) AddImageBuildResolver(r ImageBuildResolver) { c.imgres = append(c.imgres, r) }
func (c *containerEnv) SetContainerNamePrefix(p string)            { c.prefix = p }

func (c *containerEnv) Init() (err error) {
	if c.driver == nil {
		c.driver, err = driver.New(c.dtype)
	}
	return err
}

func (c *containerEnv) Execute(ctx context.Context, a *Action) (err error) {
	ctx, cancelFn := context.WithCancel(ctx)
	defer cancelFn()
	if err = c.Init(); err != nil {
		return err
	}
	streams := a.GetInput().IO
	actConf := a.ActionDef()
	log.Debug("Starting execution of the action %q in %q environment, command %v", a.ID, c.dtype, actConf.Command)
	// @todo consider reusing the same container and run exec
	name := genContainerName(a, c.prefix, nil)
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
		name = genContainerName(a, c.prefix, exMap)
	}

	// Create container.
	runConfig := &types.ContainerCreateOptions{
		ContainerName: name,
		ExtraHosts:    actConf.ExtraHosts,
		AutoRemove:    true,
		OpenStdin:     true,
		StdinOnce:     true,
		AttachStdin:   true,
		AttachStdout:  true,
		AttachStderr:  true,
		Tty:           streams.In().IsTerminal(),
		Env:           actConf.Env,
		User:          getCurrentUser(),
	}
	cid, err := c.containerCreate(ctx, a, runConfig)
	if err != nil {
		return err
	}
	if cid == "" {
		return errors.New("error on creating a container")
	}

	// Check if TTY was requested, but not supported.
	if ttyErr := streams.In().CheckTty(runConfig.AttachStdin, runConfig.Tty); ttyErr != nil {
		return ttyErr
	}

	if !runConfig.Tty {
		sigc := notifyAllSignals()
		go ForwardAllSignals(ctx, c.driver, cid, sigc)
		defer signal.StopCatch(sigc)
	}

	// Attach streams to the terminal.
	cio, errCh, err := c.attachContainer(ctx, streams, cid, runConfig)
	if err != nil {
		return err
	}
	defer func() {
		_ = cio.Close()
	}()
	statusCh := c.containerWait(ctx, cid, runConfig)

	// Start the container
	if err = c.driver.ContainerStart(ctx, cid, types.ContainerStartOptions{}); err != nil {
		cancelFn()
		<-errCh
		if runConfig.AutoRemove {
			<-statusCh
		}
		return err
	}

	// Resize TTY on window resize.
	if runConfig.Tty {
		if err = driver.MonitorTtySize(ctx, c.driver, streams, cid, false); err != nil {
			log.Err("Error monitoring TTY size:", err)
		}
	}

	if errCh != nil {
		if err = <-errCh; err != nil {
			if _, ok := err.(term.EscapeError); ok {
				// The user entered the detach escape sequence.
				return nil
			}

			log.Debug("Error hijack: %s", err)
			return err
		}
	}

	status := <-statusCh
	// @todo maybe we should note that SIG was sent to the container. Code 130 is sent on Ctlr+C.
	log.Info("action %q finished with the exit code %d", a.ID, status)
	return nil
}

func getCurrentUser() string {
	curuser := ""
	// If running in a container native environment, run container as a current user.
	// @todo review, it won't work with a remote context.
	if runtime.GOOS == "linux" {
		u, err := osuser.Current()
		if err == nil {
			curuser = u.Uid + ":" + u.Gid
		}
	}
	return curuser
}

func genContainerName(a *Action, prefix string, existing map[string]struct{}) string {
	// Replace command name "-", ":", and "." to "_".
	var rpl = strings.NewReplacer("-", "_", ":", "_", ".", "_")
	base := prefix + rpl.Replace(a.ID)
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

func (c *containerEnv) Close() error {
	return c.driver.Close()
}

func (c *containerEnv) imageEnsure(ctx context.Context, a *Action) error {
	image := a.ActionDef().Image
	// Prepend action to have the top priority in image build resolution.
	r := ChainImageBuildResolver{append(ChainImageBuildResolver{a}, c.imgres...)}
	status, err := c.driver.ImageEnsure(ctx, types.ImageOptions{
		Name:  image,
		Build: r.ImageBuildInfo(image),
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
		log.Debug("Pulling %q progress", image)
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
		log.Debug("Building %q progress", image)
		scanner := bufio.NewScanner(status.Progress)
		for scanner.Scan() {
			log.Debug(scanner.Text())
		}
	}

	return nil
}

func (c *containerEnv) containerCreate(ctx context.Context, a *Action, opts *types.ContainerCreateOptions) (string, error) {
	if err := c.imageEnsure(ctx, a); err != nil {
		return "", err
	}

	// Create a container
	actConf := a.ActionDef()
	cid, err := c.driver.ContainerCreate(ctx, types.ContainerCreateOptions{
		ContainerName: opts.ContainerName,
		Image:         actConf.Image,
		Cmd:           actConf.Command,
		WorkingDir:    containerHostMount,
		Binds: map[string]string{
			".":     containerHostMount,
			a.Dir(): containerActionMount,
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
		User:         opts.User,
	})
	if err != nil {
		return "", err
	}

	return cid, nil
}

func (c *containerEnv) containerWait(ctx context.Context, cid string, opts *types.ContainerCreateOptions) <-chan int {
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

func (c *containerEnv) attachContainer(ctx context.Context, streams cli.Streams, cid string, opts *types.ContainerCreateOptions) (io.Closer, <-chan error, error) {
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
		errCh <- driver.ContainerIOStream(ctx, streams, cio, opts)
	}()
	return cio, errCh, nil
}
