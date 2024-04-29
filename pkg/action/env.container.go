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
	// Container mount paths.
	containerHostMount   = "/host"
	containerActionMount = "/action"

	// Environment specific flags.
	containerFlagUseVolumeWD = "use-volume-wd"
	containerFlagRemoveImage = "remove-image"
	containerFlagNoCache     = "no-cache"
	containerFlagEntrypoint  = "entrypoint"
	containerFlagExec        = "exec"
)

type containerEnv struct {
	driver driver.ContainerRunner
	dtype  driver.Type

	// Container related functionality extenders
	imgres   ChainImageBuildResolver
	imgccres *ImageBuildCacheResolver
	nameprv  ContainerNameProvider

	// Runtime flags
	useVolWD      bool
	removeImg     bool
	noCache       bool
	entrypoint    string
	entrypointSet bool
	exec          bool
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
		&Option{
			Name:        containerFlagNoCache,
			Title:       "No cache",
			Description: "Send command to build container without cache",
			Type:        jsonschema.Boolean,
			Default:     false,
		},
		&Option{
			Name:        containerFlagEntrypoint,
			Title:       "Image Entrypoint",
			Description: "Overwrite the default ENTRYPOINT of the image",
			Type:        jsonschema.String,
			Default:     "",
		},
		&Option{
			Name:        containerFlagExec,
			Title:       "Exec command",
			Description: "Overwrite CMD definition of the container",
			Type:        jsonschema.Boolean,
			Default:     false,
		},
	}
}

func (c *containerEnv) UseFlags(flags TypeOpts) error {
	if v, ok := flags[containerFlagUseVolumeWD]; ok {
		c.useVolWD = v.(bool)
	}

	if r, ok := flags[containerFlagRemoveImage]; ok {
		c.removeImg = r.(bool)
	}

	if nc, ok := flags[containerFlagNoCache]; ok {
		c.noCache = nc.(bool)
	}

	if e, ok := flags[containerFlagEntrypoint]; ok {
		c.entrypointSet = true
		c.entrypoint = e.(string)
	}

	if ex, ok := flags[containerFlagExec]; ok {
		c.exec = ex.(bool)
	}

	return nil
}

func (c *containerEnv) ValidateInput(a Action, args TypeArgs) error {
	if c.exec {
		return nil
	}

	act, ok := a.(*ContainerAction)
	if !ok {
		panic("not supported action type submitted to container env")
	}

	// Check arguments if no exec flag present.
	return act.ValidateInput(args)
}

func (c *containerEnv) AddImageBuildResolver(r ImageBuildResolver)            { c.imgres = append(c.imgres, r) }
func (c *containerEnv) SetImageBuildCacheResolver(s *ImageBuildCacheResolver) { c.imgccres = s }
func (c *containerEnv) SetContainerNameProvider(p ContainerNameProvider)      { c.nameprv = p }

// Init prepares the run environment.
func (c *containerEnv) Init() (err error) {
	if c.driver == nil {
		c.driver, err = driver.New(c.dtype)
	}
	return err
}

// Execute runs action a in the environment and operates with IO through streams.
func (c *containerEnv) Execute(ctx context.Context, a Action) (err error) {
	ctx, cancelFn := context.WithCancel(ctx)
	defer cancelFn()
	if err = c.Init(); err != nil {
		return err
	}

	act, ok := a.(*ContainerAction)
	if !ok {
		panic("not supported action type submitted to container env")
	}

	streams := act.GetInput().IO
	actConf := act.ActionDef()
	log.Debug("Starting execution of the action %q in %q environment, command %v", act.GetID(), c.dtype, actConf.Command)
	// @todo consider reusing the same container and run exec
	name := c.nameprv.Get(act.GetID())
	existing := c.driver.ContainerList(ctx, types.ContainerListOptions{SearchName: name})
	if len(existing) > 0 {
		return fmt.Errorf("the action %q can't start, the container name is in use, please, try again", act.GetID())
	}

	var autoRemove = true
	if c.useVolWD {
		// Do not remove the volume until we copy the data back.
		autoRemove = false
	}

	// Add entrypoint command option.
	var entrypoint []string
	if c.entrypointSet {
		entrypoint = []string{c.entrypoint}
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
		Entrypoint:    entrypoint,
	}
	log.Debug("Creating a container for action %q", act.GetID())
	cid, err := c.containerCreate(ctx, act, runConfig)
	if err != nil {
		return err
	}
	if cid == "" {
		return errors.New("error on creating a container")
	}

	log.Debug("Successfully created container %q for action %q", cid, act.GetID())
	// Copy working dirs to the container.
	if c.useVolWD {
		// @todo test somehow.
		cli.Println(`Flag "--%s" is set. Copying the working directory inside the container.`, containerFlagUseVolumeWD)
		err = c.copyDirToContainer(ctx, cid, act.WorkDir(), containerHostMount)
		if err != nil {
			return err
		}
		err = c.copyDirToContainer(ctx, cid, act.Dir(), containerActionMount)
		if err != nil {
			return err
		}
	}

	// Check if TTY was requested, but not supported.
	if ttyErr := streams.In().CheckTty(runConfig.AttachStdin, runConfig.Tty); ttyErr != nil {
		return ttyErr
	}

	if !runConfig.Tty {
		log.Debug("Start watching signals %q, action %q", cid, act.GetID())
		sigc := notifyAllSignals()
		go ForwardAllSignals(ctx, c.driver, cid, sigc)
		defer signal.StopCatch(sigc)
	}

	// Attach streams to the terminal.
	log.Debug("Attaching streams of %q, action %q", cid, act.GetID())
	cio, errCh, err := c.attachContainer(ctx, streams, cid, runConfig)
	if err != nil {
		return err
	}
	defer func() {
		_ = cio.Close()
	}()
	log.Debug("Watching status of %q, action %q", cid, act.GetID())
	statusCh := c.containerWait(ctx, cid, runConfig)

	// Start the container
	log.Debug("Starting container %q, action %q", cid, act.GetID())
	if err = c.driver.ContainerStart(ctx, cid, types.ContainerStartOptions{}); err != nil {
		log.Debug("Failed starting the container %q, action %q", cid, act.GetID())
		cancelFn()
		<-errCh
		if runConfig.AutoRemove {
			<-statusCh
		}
		return err
	}

	// Resize TTY on window resize.
	if runConfig.Tty {
		log.Debug("Watching TTY resize %q, action %q", cid, act.GetID())
		if err = driver.MonitorTtySize(ctx, c.driver, streams, cid, false); err != nil {
			log.Err("Error monitoring TTY size:", err)
		}
	}

	log.Debug("Waiting execution of %q, action %q", cid, act.GetID())
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
	msg := fmt.Sprintf("action %q finished with the exit code %d", act.GetID(), status)
	log.Info(msg)
	if status != 0 {
		err = RunStatusError{code: status, msg: msg}
	}

	// Copy back the result from the volume.
	// @todo it's a bad implementation considering consequential runs, need to find a better way to sync with remote.
	if c.useVolWD {
		path := act.WorkDir()
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

	defer func() {
		if !c.removeImg {
			return
		}
		log.Debug("Removing container %q, action %q", cid, act.GetID())
		errImg := c.imageRemove(ctx, act)
		if errImg != nil {
			log.Err("Image remove returned an error: %v", errImg)
		} else {
			cli.Println("Image %q was successfully removed", act.ActionDef().Image)
		}
	}()

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

// Close does wrap up operations.
func (c *containerEnv) Close() error {
	return c.driver.Close()
}

func (c *containerEnv) imageRemove(ctx context.Context, a *ContainerAction) error {
	_, err := c.driver.ImageRemove(ctx, a.ActionDef().Image, types.ImageRemoveOptions{
		Force:         true,
		PruneChildren: false,
	})

	return err
}

func (c *containerEnv) isRebuildRequired(bi *types.BuildDefinition) (bool, error) {
	// @todo test image cache resolution somehow.
	if c.imgccres == nil || bi == nil {
		return false, nil
	}

	err := c.imgccres.EnsureLoaded()
	if err != nil {
		return false, err
	}

	dirSum, err := c.imgccres.DirHash(bi.Context)
	if err != nil {
		return false, err
	}

	doRebuild := false
	for _, tag := range bi.Tags {
		sum := c.imgccres.GetSum(tag)
		if sum != dirSum {
			c.imgccres.SetSum(tag, dirSum)
			doRebuild = true
		}
	}

	if errCache := c.imgccres.Save(); errCache != nil {
		log.Warn("Failed to update actions.sum file: %v", errCache)
	}

	return doRebuild, nil
}

func (c *containerEnv) imageEnsure(ctx context.Context, a *ContainerAction) error {
	streams := a.GetInput().IO
	image := a.ActionDef().Image
	// Prepend action to have the top priority in image build resolution.
	r := ChainImageBuildResolver{append(ChainImageBuildResolver{a}, c.imgres...)}

	buildInfo := r.ImageBuildInfo(image)
	forceRebuild, err := c.isRebuildRequired(buildInfo)
	if err != nil {
		return err
	}

	status, err := c.driver.ImageEnsure(ctx, types.ImageOptions{
		Name:         image,
		Build:        buildInfo,
		NoCache:      c.noCache,
		ForceRebuild: forceRebuild,
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

func (c *containerEnv) containerCreate(ctx context.Context, a *ContainerAction, opts *types.ContainerCreateOptions) (string, error) {
	if err := c.imageEnsure(ctx, a); err != nil {
		return "", err
	}

	// Create a container
	actConf := a.ActionDef()

	// Override Cmd with exec command.
	if c.exec {
		actConf.Command = a.GetInput().ArgsRaw
	}

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
		Entrypoint:    opts.Entrypoint,
	}

	if c.useVolWD {
		// Use anonymous volumes to be removed after finish.
		createOpts.Volumes = map[string]struct{}{
			containerHostMount:   {},
			containerActionMount: {},
		}
	} else {
		createOpts.Binds = []string{
			absPath(a.WorkDir()) + ":" + containerHostMount,
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
