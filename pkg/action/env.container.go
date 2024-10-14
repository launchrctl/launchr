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

	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/namesgenerator"
	"github.com/moby/sys/signal"
	"github.com/moby/term"

	"github.com/launchrctl/launchr/internal/launchr"
	"github.com/launchrctl/launchr/pkg/driver"
	"github.com/launchrctl/launchr/pkg/jsonschema"
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
	driver  driver.ContainerRunner
	dtype   driver.Type
	logWith []any

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
	return &containerEnv{
		dtype:   t,
		nameprv: ContainerNameProvider{Prefix: "launchr_", RandomSuffix: true},
	}
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
func (c *containerEnv) ValidateInput(a *Action, args TypeArgs) error {
	if c.exec {
		return nil
	}

	// Check arguments if no exec flag present.
	return a.ValidateInput(args)
}
func (c *containerEnv) AddImageBuildResolver(r ImageBuildResolver)            { c.imgres = append(c.imgres, r) }
func (c *containerEnv) SetImageBuildCacheResolver(s *ImageBuildCacheResolver) { c.imgccres = s }
func (c *containerEnv) SetContainerNameProvider(p ContainerNameProvider)      { c.nameprv = p }

func (c *containerEnv) Init(_ context.Context) (err error) {
	c.logWith = nil
	if c.driver == nil {
		c.driver, err = driver.New(c.dtype)
	}
	return err
}

func (c *containerEnv) log(attrs ...any) *launchr.Slog {
	if attrs != nil {
		c.logWith = append(c.logWith, attrs...)
	}
	return launchr.Log().With(c.logWith...)
}

func (c *containerEnv) Execute(ctx context.Context, a *Action) (err error) {
	ctx, cancelFn := context.WithCancel(ctx)
	defer cancelFn()
	if err = c.Init(ctx); err != nil {
		return err
	}
	streams := a.GetInput().IO
	actConf := a.ActionDef()
	log := c.log("run_env", c.dtype, "action_id", a.ID, "image", actConf.Image, "command", actConf.Command)
	log.Debug("starting execution of the action")
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
	log.Debug("creating a container for an action")
	cid, err := c.containerCreate(ctx, a, runConfig)
	if err != nil {
		return err
	}
	if cid == "" {
		return errors.New("error on creating a container")
	}

	log = c.log("container_id", cid)
	log.Debug("successfully created a container for an action")
	// Copy working dirs to the container.
	if c.useVolWD {
		// @todo test somehow.
		launchr.Term().Info().Printfln(`Flag "--%s" is set. Copying the working directory inside the container.`, containerFlagUseVolumeWD)
		err = c.copyDirToContainer(ctx, cid, a.WorkDir(), containerHostMount)
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
		log.Debug("watching container signals")
		sigc := notifyAllSignals()
		go ForwardAllSignals(ctx, c.driver, cid, sigc)
		defer signal.StopCatch(sigc)
	}

	// Attach streams to the terminal.
	log.Debug("attaching container streams")
	cio, errCh, err := c.attachContainer(ctx, streams, cid, runConfig)
	if err != nil {
		return err
	}
	defer func() {
		_ = cio.Close()
	}()
	log.Debug("watching run status of container")
	statusCh := c.containerWait(ctx, cid, runConfig)

	// Start the container
	log.Debug("starting container")
	if err = c.driver.ContainerStart(ctx, cid, types.ContainerStartOptions{}); err != nil {
		log.Debug("failed starting the container")
		cancelFn()
		<-errCh
		if runConfig.AutoRemove {
			<-statusCh
		}
		return err
	}

	// Resize TTY on window resize.
	if runConfig.Tty {
		log.Debug("watching TTY resize")
		if err = driver.MonitorTtySize(ctx, c.driver, streams, cid, false); err != nil {
			log.Error("error monitoring tty size", "error", err)
		}
	}

	log.Debug("waiting execution of the container")
	if errCh != nil {
		if err = <-errCh; err != nil {
			if _, ok := err.(term.EscapeError); ok {
				// The user entered the detach escape sequence.
				return nil
			}

			log.Debug("error hijack", "error", err)
			return err
		}
	}

	status := <-statusCh
	// @todo maybe we should note that SIG was sent to the container. Code 130 is sent on Ctlr+C.
	log.Info("action finished with the exit code", "exit_code", status)
	if status != 0 {
		err = RunStatusError{code: status, actionID: a.ID}
	}

	// Copy back the result from the volume.
	// @todo it's a bad implementation considering consequential runs, need to find a better way to sync with remote.
	if c.useVolWD {
		path := a.WorkDir()
		launchr.Term().Info().Printfln(`Flag "--%s" is set. Copying back the result of the action run.`, containerFlagUseVolumeWD)
		err = c.copyFromContainer(ctx, cid, containerHostMount, filepath.Dir(path), filepath.Base(path))
		defer func() {
			err = c.driver.ContainerRemove(ctx, cid, types.ContainerRemoveOptions{})
			if err != nil {
				log.Error("error on cleaning the running environment", "error", err)
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
		log.Debug("removing container image after run")
		errImg := c.imageRemove(ctx, a)
		if errImg != nil {
			log.Error("failed to remove image", "error", errImg)
		} else {
			log.Debug("image was successfully removed")
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

func (c *containerEnv) Close() error {
	return c.driver.Close()
}

func (c *containerEnv) imageRemove(ctx context.Context, a *Action) error {
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
		c.log().Warn("failed to update actions.sum file", "error", errCache)
	}

	return doRebuild, nil
}

func (c *containerEnv) imageEnsure(ctx context.Context, a *Action) error {
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

	log := c.log()
	switch status.Status {
	case types.ImageExists:
		log.Debug("image exists locally")
	case types.ImagePull:
		if status.Progress == nil {
			break
		}
		defer func() {
			_ = status.Progress.Close()
		}()
		launchr.Term().Printfln("Image %q doesn't exist locally, pulling from the registry...", image)
		log.Info("image doesn't exist locally, pulling from the registry")
		// Output docker status only in Debug.
		err = displayJSONMessages(status.Progress, streams)
		if err != nil {
			launchr.Term().Error().Println("Error occurred while pulling the image %q", image)
			log.Error("error while pulling the image", "error", err)
		}
	case types.ImageBuild:
		if status.Progress == nil {
			break
		}
		defer func() {
			_ = status.Progress.Close()
		}()
		launchr.Term().Printfln("Image %q doesn't exist locally, building...", image)
		log.Info("image doesn't exist locally, building the image")
		// Output docker status only in Debug.
		err = displayJSONMessages(status.Progress, streams)
		if err != nil {
			launchr.Term().Error().Println("Error occurred while building the image %q", image)
			log.Error("error while building the image", "error", err)
		}
	}

	return err
}

func displayJSONMessages(in io.Reader, streams launchr.Streams) error {
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
		flags := ""
		// Check SELinux settings to allow reading the FS inside a container.
		if c.isSELinuxEnabled(ctx) {
			// Use the lowercase z flag to allow concurrent actions access to the FS.
			flags += ":z"
			launchr.Term().Warning().Printfln(
				"SELinux is detected. The volumes will be mounted with the %q flags, which will relabel your files.\n"+
					"This process may take time or potentially break existing permissions.",
				flags,
			)
			c.log().Warn("using selinux flags", "flags", flags)
		}
		createOpts.Binds = []string{
			absPath(a.WorkDir()) + ":" + containerHostMount + flags,
			absPath(a.Dir()) + ":" + containerActionMount + flags,
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
	log := c.log()
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
			log.Error("error waiting for container", "error", err)
			statusC <- 125
		case res := <-resCh:
			if res.Error != nil {
				log.Error("error in container run", "error", res.Error)
				statusC <- 125
			} else {
				log.Debug("received run status code", "exit_code", res.StatusCode)
				statusC <- res.StatusCode
			}
		case <-ctx.Done():
			log.Debug("stop waiting for container on context finish")
			statusC <- 125
		}
	}()

	return statusC
}

func (c *containerEnv) attachContainer(ctx context.Context, streams launchr.Streams, cid string, opts *types.ContainerCreateOptions) (io.Closer, <-chan error, error) {
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

func (c *containerEnv) isSELinuxEnabled(ctx context.Context) bool {
	// First, we check if it's enabled at the OS level, then if it's enabled in the container runner.
	// If the feature is not enabled in the runner environment,
	// containers will bypass SELinux and will function as if SELinux is disabled in the OS.
	d, ok := c.driver.(driver.ContainerRunnerSELinux)
	return ok && launchr.IsSELinuxEnabled() && d.IsSELinuxSupported(ctx)
}
