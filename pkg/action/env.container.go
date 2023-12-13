package action

import (
	"context"
	"errors"
	"fmt"
	"io"
	osuser "os/user"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/moby/moby/pkg/archive"
	"github.com/moby/moby/pkg/jsonmessage"
	"github.com/moby/moby/pkg/namesgenerator"
	"github.com/moby/sys/signal"
	"github.com/moby/term"

	"github.com/launchrctl/launchr/pkg/cli"
	"github.com/launchrctl/launchr/pkg/driver"
	"github.com/launchrctl/launchr/pkg/jsonschema"
	"github.com/launchrctl/launchr/pkg/log"
	"github.com/launchrctl/launchr/pkg/types"
)

const (
	containerHostMount   = "/host"
	containerActionMount = "/action"

	// Environment specific flags.
	containerFlagUseVolumeWD = "use-volume-wd"
	containerFlagRemoveImage = "remove-image"
)

// ContainerExecError is an execution error also containing command exit code.
type ContainerExecError struct {
	code int
	msg  string
}

func (e ContainerExecError) Error() string {
	return e.msg
}

// GetCode returns executions exit code.
func (e ContainerExecError) GetCode() int {
	return e.code
}

type containerEnv struct {
	driver  driver.ContainerRunner
	imgres  ChainImageBuildResolver
	dtype   driver.Type
	nameprv ContainerNameProvider

	// Runtime flags
	useVolWD  bool
	removeImg bool
}

// ContainerNameProvider provides an ability to generate a random container name
type ContainerNameProvider struct {
	Prefix       string
	RandomSuffix bool
}

// Get generates a new container name
func (p ContainerNameProvider) Get(name string) string {
	var rpl = strings.NewReplacer("-", "_", ":", "_", ".", "_")
	suffix := ""
	if p.RandomSuffix {
		suffix = "_" + namesgenerator.GetRandomName(0)
	}

	return p.Prefix + rpl.Replace(name) + suffix
}

// NewDockerEnvironment creates a new action Docker environment.
func NewDockerEnvironment() RunEnvironment {
	return NewContainerEnvironment(driver.Docker)
}

// NewContainerEnvironment creates a new action container run environment.
func NewContainerEnvironment(t driver.Type) RunEnvironment {
	return &containerEnv{dtype: t, nameprv: ContainerNameProvider{Prefix: "launchr_", RandomSuffix: true}}
}

func (c *containerEnv) FlagsDefinition() OptionsList {
	return OptionsList{
		&Option{
			Name:        containerFlagUseVolumeWD,
			Title:       "Use volume as a WD",
			Description: "Copy the working directory to a container volume and not bind local paths. Usually used with remote environments.",
			Type:        jsonschema.Boolean,
			Default:     false,
		},
		&Option{
			Name:        containerFlagRemoveImage,
			Title:       "Remove Image",
			Description: "Remove an image after execution of action",
			Type:        jsonschema.Boolean,
			Default:     false,
		},
	}
}

func (c *containerEnv) UseFlags(flags TypeOpts) error {
	if v, ok := flags[containerFlagUseVolumeWD]; ok {
		c.useVolWD = v.(bool)
	}

	if v, ok := flags[containerFlagRemoveImage]; ok {
		c.removeImg = v.(bool)
	}

	return nil
}

func (c *containerEnv) AddImageBuildResolver(r ImageBuildResolver)       { c.imgres = append(c.imgres, r) }
func (c *containerEnv) SetContainerNameProvider(p ContainerNameProvider) { c.nameprv = p }

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
	name := c.nameprv.Get(a.ID)
	existing := c.driver.ContainerList(ctx, types.ContainerListOptions{SearchName: name})
	if len(existing) > 0 {
		return fmt.Errorf("the action %q can't start, the container name is in use, please, try again", a.ID)
	}

	var autoRemove = true
	if c.useVolWD {
		// Do not remove the volume until we copy the data back.
		autoRemove = false
	}

	// Create container.
	runConfig := &types.ContainerCreateOptions{
		ContainerName: name,
		ExtraHosts:    actConf.ExtraHosts,
		AutoRemove:    autoRemove,
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
	// Copy working dirs to the container.
	if c.useVolWD {
		// @todo test somehow.
		cli.Println(`Flag "--%s" is set. Copying the working directory inside the container.`, containerFlagUseVolumeWD)
		err = c.copyDirToContainer(ctx, cid, ".", containerHostMount)
		if err != nil {
			return err
		}
		err = c.copyDirToContainer(ctx, cid, a.Dir(), containerActionMount)
		if err != nil {
			return err
		}
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
	msg := fmt.Sprintf("action %q finished with the exit code %d", a.ID, status)
	if status != 0 {
		err = ContainerExecError{code: status, msg: msg}
	} else {
		log.Info(msg)
	}

	// Copy back the result from the volume.
	// @todo it's a bad implementation considering consequential runs, need to find a better way to sync with remote.
	if c.useVolWD {
		path := absPath(".")
		cli.Println(`Flag "--%s" is set. Copying back the result of the action run.`, containerFlagUseVolumeWD)
		err = c.copyFromContainer(ctx, cid, containerHostMount, filepath.Dir(path), filepath.Base(path))
		defer func() {
			err = c.driver.ContainerRemove(ctx, cid, types.ContainerRemoveOptions{})
			if err != nil {
				log.Err("Error on cleaning the running environment: %v", err)
			}
		}()
		if err != nil {
			return err
		}
	}

	if c.removeImg {
		err = c.imageRemove(ctx, a)
		if err != nil {
			log.Err("Image remove returned an error: %v", err)
		} else {
			cli.Println("Image %q was successfully removed", a.ActionDef().Image)
		}
	}

	return err
}

func getCurrentUser() string {
	curuser := ""
	// If running in a container native environment, run container as a current user.
	// @todo review, it won't work with a remote context.
	switch runtime.GOOS {
	case "linux", "darwin":
		u, err := osuser.Current()
		if err == nil {
			curuser = u.Uid + ":" + u.Gid
		}
	}
	return curuser
}

func (c *containerEnv) Close() error {
	return c.driver.Close()
}

func (c *containerEnv) imageRemove(ctx context.Context, a *Action) error {
	_, err := c.driver.ImageRemove(ctx, a.ActionDef().Image, types.ImageRemoveOptions{
		Force:         false,
		PruneChildren: false,
	})

	return err
}

func (c *containerEnv) imageEnsure(ctx context.Context, a *Action) error {
	streams := a.GetInput().IO
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
		err = displayJSONMessages(status.Progress, streams)
		if err != nil {
			cli.Println("There was an error while pulling the image")
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
		err = displayJSONMessages(status.Progress, streams)
		if err != nil {
			cli.Println("There was an error while building the image")
		}
	}

	return err
}

func displayJSONMessages(in io.Reader, streams cli.Streams) error {
	err := jsonmessage.DisplayJSONMessagesToStream(in, streams.Out(), nil)
	if err != nil {
		if jerr, ok := err.(*jsonmessage.JSONError); ok {
			// If no error code is set, default to 1
			if jerr.Code == 0 {
				jerr.Code = 1
			}
			return jerr
		}
	}
	return err
}

func (c *containerEnv) containerCreate(ctx context.Context, a *Action, opts *types.ContainerCreateOptions) (string, error) {
	if err := c.imageEnsure(ctx, a); err != nil {
		return "", err
	}

	// Create a container
	actConf := a.ActionDef()
	createOpts := types.ContainerCreateOptions{
		ContainerName: opts.ContainerName,
		Image:         actConf.Image,
		Cmd:           actConf.Command,
		WorkingDir:    containerHostMount,
		NetworkMode:   types.NetworkModeHost,
		ExtraHosts:    opts.ExtraHosts,
		AutoRemove:    opts.AutoRemove,
		OpenStdin:     opts.OpenStdin,
		StdinOnce:     opts.StdinOnce,
		AttachStdin:   opts.AttachStdin,
		AttachStdout:  opts.AttachStdout,
		AttachStderr:  opts.AttachStderr,
		Tty:           opts.Tty,
		Env:           opts.Env,
		User:          opts.User,
	}

	if c.useVolWD {
		// Use anonymous volumes to be removed after finish.
		createOpts.Volumes = map[string]struct{}{
			containerHostMount:   {},
			containerActionMount: {},
		}
	} else {
		createOpts.Binds = []string{
			absPath(".") + ":" + containerHostMount,
			absPath(a.Dir()) + ":" + containerActionMount,
		}
	}
	cid, err := c.driver.ContainerCreate(ctx, createOpts)
	if err != nil {
		return "", err
	}

	return cid, nil
}

func absPath(src string) string {
	abs, err := filepath.Abs(filepath.Clean(src))
	if err != nil {
		panic(err)
	}
	return abs
}

// copyDirToContainer copies dir content to a container.
func (c *containerEnv) copyDirToContainer(ctx context.Context, cid, srcPath, dstPath string) error {
	return c.copyToContainer(ctx, cid, srcPath, filepath.Dir(dstPath), filepath.Base(dstPath))
}

// copyToContainer copies dir/file to a container. Directory will be copied as a subdirectory.
func (c *containerEnv) copyToContainer(ctx context.Context, cid, srcPath, dstPath, rebaseName string) error {
	// Prepare destination copy info by stat-ing the container path.
	dstInfo := archive.CopyInfo{Path: dstPath}
	dstStat, err := c.driver.ContainerStatPath(ctx, cid, dstPath)
	if err != nil {
		return err
	}
	dstInfo.Exists, dstInfo.IsDir = true, dstStat.Mode.IsDir()

	// Prepare source copy info.
	srcInfo, err := archive.CopyInfoSourcePath(absPath(srcPath), false)
	if err != nil {
		return err
	}
	srcInfo.RebaseName = rebaseName

	srcArchive, err := archive.TarResource(srcInfo)
	if err != nil {
		return err
	}
	defer srcArchive.Close()

	dstDir, preparedArchive, err := archive.PrepareArchiveCopy(srcArchive, srcInfo, dstInfo)
	if err != nil {
		return err
	}
	defer preparedArchive.Close()

	options := types.CopyToContainerOptions{
		AllowOverwriteDirWithFile: false,
		CopyUIDGID:                false,
	}
	return c.driver.CopyToContainer(ctx, cid, dstDir, preparedArchive, options)
}

func resolveLocalPath(localPath string) (absPath string, err error) {
	if absPath, err = filepath.Abs(localPath); err != nil {
		return
	}
	return archive.PreserveTrailingDotOrSeparator(absPath, localPath), nil
}

func (c *containerEnv) copyFromContainer(ctx context.Context, cid, srcPath, dstPath, rebaseName string) (err error) {
	// Get an absolute destination path.
	dstPath, err = resolveLocalPath(dstPath)
	if err != nil {
		return err
	}

	content, stat, err := c.driver.CopyFromContainer(ctx, cid, srcPath)
	if err != nil {
		return err
	}
	defer content.Close()

	srcInfo := archive.CopyInfo{
		Path:       srcPath,
		Exists:     true,
		IsDir:      stat.Mode.IsDir(),
		RebaseName: rebaseName,
	}

	preArchive := content
	if len(srcInfo.RebaseName) != 0 {
		_, srcBase := archive.SplitPathDirEntry(srcInfo.Path)
		preArchive = archive.RebaseArchiveEntries(content, srcBase, srcInfo.RebaseName)
	}

	return archive.CopyTo(preArchive, srcInfo, dstPath)
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
		Stream: true,
		Stdin:  opts.AttachStdin,
		Stdout: opts.AttachStdout,
		Stderr: opts.AttachStderr,
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
