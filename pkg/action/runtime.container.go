package action

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	osuser "os/user"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/launchrctl/launchr/internal/launchr"
	"github.com/launchrctl/launchr/pkg/archive"
	"github.com/launchrctl/launchr/pkg/driver"
	"github.com/launchrctl/launchr/pkg/jsonschema"
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

type runtimeContainer struct {
	// crt is a container runtime.
	crt driver.ContainerRunner
	// rtype is a container runtime type string.
	rtype driver.Type
	// logWith contains context arguments for a structured logger.
	logWith []any

	// Container related functionality extenders
	// @todo migrate to events/hooks for loose coupling.
	imgres   ChainImageBuildResolver
	imgccres *ImageBuildCacheResolver
	nameprv  ContainerNameProvider

	// Runtime flags
	useVolWD      bool // Deprecated: with no replacement.
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
		suffix = "_" + driver.GetRandomName(0)
	}

	return p.Prefix + rpl.Replace(name) + suffix
}

// NewContainerRuntimeDocker creates a new action Docker runtime.
func NewContainerRuntimeDocker() ContainerRuntime {
	return NewContainerRuntime(driver.Docker)
}

// NewContainerRuntimeKubernetes creates a new action Kubernetes runtime.
func NewContainerRuntimeKubernetes() ContainerRuntime {
	return NewContainerRuntime(driver.Kubernetes)
}

// NewContainerRuntime creates a new action container runtime.
func NewContainerRuntime(t driver.Type) ContainerRuntime {
	return &runtimeContainer{
		rtype:   t,
		nameprv: ContainerNameProvider{Prefix: "launchr_", RandomSuffix: true},
	}
}

func (c *runtimeContainer) Clone() Runtime {
	return NewContainerRuntime(c.rtype)
}

func (c *runtimeContainer) FlagsDefinition() ParametersList {
	return ParametersList{
		&DefParameter{
			Name:        containerFlagUseVolumeWD,
			Title:       "Use volume as a WD",
			Description: "Copy the working directory to a container volume and not bind local paths. Usually used with remote environments.",
			Type:        jsonschema.Boolean,
			Default:     false,
		},
		&DefParameter{
			Name:        containerFlagRemoveImage,
			Title:       "Remove Image",
			Description: "Remove an image after execution of action",
			Type:        jsonschema.Boolean,
			Default:     false,
		},
		&DefParameter{
			Name:        containerFlagNoCache,
			Title:       "No cache",
			Description: "Send command to build container without cache",
			Type:        jsonschema.Boolean,
			Default:     false,
		},
		&DefParameter{
			Name:        containerFlagEntrypoint,
			Title:       "Image Entrypoint",
			Description: "Overwrite the default ENTRYPOINT of the image",
			Type:        jsonschema.String,
			Default:     "",
		},
		&DefParameter{
			Name:        containerFlagExec,
			Title:       "Exec command",
			Description: "Overwrite CMD definition of the container",
			Type:        jsonschema.Boolean,
			Default:     false,
		},
	}
}

func (c *runtimeContainer) UseFlags(flags InputParams) error {
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
func (c *runtimeContainer) ValidateInput(_ *Action, input *Input) error {
	if c.exec {
		// Mark input as validated because arguments are passed directly to exec.
		input.SetValidated(true)
	}
	return nil
}
func (c *runtimeContainer) AddImageBuildResolver(r ImageBuildResolver) {
	c.imgres = append(c.imgres, r)
}
func (c *runtimeContainer) SetImageBuildCacheResolver(s *ImageBuildCacheResolver) { c.imgccres = s }
func (c *runtimeContainer) SetContainerNameProvider(p ContainerNameProvider)      { c.nameprv = p }

func (c *runtimeContainer) Init(_ context.Context, _ *Action) (err error) {
	c.logWith = nil
	if c.crt == nil {
		c.crt, err = driver.New(c.rtype)
	}
	return err
}

func (c *runtimeContainer) log(attrs ...any) *launchr.Slog {
	if attrs != nil {
		c.logWith = append(c.logWith, attrs...)
	}
	return launchr.Log().With(c.logWith...)
}

func (c *runtimeContainer) Execute(ctx context.Context, a *Action) (err error) {
	ctx, cancelFn := context.WithCancel(ctx)
	defer cancelFn()
	streams := a.Input().Streams()
	runDef := a.RuntimeDef()
	if runDef.Container == nil {
		return errors.New("action container configuration is not set, use different runtime")
	}
	log := c.log("run_env", c.rtype, "action_id", a.ID, "image", runDef.Container.Image, "command", runDef.Container.Command)
	log.Debug("starting execution of the action")
	name := c.nameprv.Get(a.ID)
	existing := c.crt.ContainerList(ctx, driver.ContainerListOptions{SearchName: name})
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
	runConfig := &driver.ContainerCreateOptions{
		ContainerName: name,
		ExtraHosts:    runDef.Container.ExtraHosts,
		AutoRemove:    autoRemove,
		OpenStdin:     true,
		StdinOnce:     true,
		AttachStdin:   true,
		AttachStdout:  true,
		AttachStderr:  true,
		Tty:           streams.In().IsTerminal(),
		Env:           runDef.Container.Env,
		User:          getCurrentUser(),
		Entrypoint:    entrypoint,
	}
	log.Debug("creating a container for an action")
	cid, err := c.containerCreate(ctx, a, runConfig)
	if err != nil {
		return fmt.Errorf("failed to create a container: %w", err)
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
			return fmt.Errorf("failed to copy host directory to the container: %w", err)
		}
		err = c.copyDirToContainer(ctx, cid, a.Dir(), containerActionMount)
		if err != nil {
			return fmt.Errorf("failed to copy action directory to the container: %w", err)
		}
	}

	// Check if TTY was requested, but not supported.
	if ttyErr := streams.In().CheckTty(runConfig.AttachStdin, runConfig.Tty); ttyErr != nil {
		return ttyErr
	}

	if !runConfig.Tty {
		log.Debug("watching container signals")
		sigc := launchr.NotifySignals()
		go launchr.HandleSignals(ctx, sigc, func(_ os.Signal, sig string) error {
			return c.crt.ContainerKill(ctx, cid, sig)
		})
		defer launchr.StopCatchSignals(sigc)
	}

	// Attach streams to the terminal.
	log.Debug("attaching container streams")
	cio, errCh, err := c.attachContainer(ctx, streams, cid, runConfig)
	if err != nil {
		return fmt.Errorf("failed to attach to the container: %w", err)
	}
	defer func() {
		_ = cio.Close()
	}()
	log.Debug("watching run status of container")
	statusCh := c.containerWait(ctx, cid, runConfig)

	// Start the container
	log.Debug("starting container")
	if err = c.crt.ContainerStart(ctx, cid, driver.ContainerStartOptions{}); err != nil {
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
		if err = driver.MonitorTtySize(ctx, c.crt, streams, cid, false); err != nil {
			log.Error("error monitoring tty size", "error", err)
		}
	}

	log.Debug("waiting execution of the container")
	if errCh != nil {
		if err = <-errCh; err != nil {
			if _, ok := err.(driver.EscapeError); ok {
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
		err = launchr.NewExitError(status, fmt.Sprintf("action %q finished with exit code %d", a.ID, status))
	}

	// Copy back the result from the volume.
	// @todo it's a bad implementation considering consequential runs, need to find a better way to sync with remote.
	if c.useVolWD {
		path := a.WorkDir()
		launchr.Term().Info().Printfln(`Flag "--%s" is set. Copying back the result of the action run.`, containerFlagUseVolumeWD)
		err = c.copyFromContainer(ctx, cid, containerHostMount, filepath.Dir(path), filepath.Base(path)+"/result")
		defer func() {
			err = c.crt.ContainerRemove(ctx, cid, driver.ContainerRemoveOptions{})
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

func (c *runtimeContainer) Close() error {
	return c.crt.Close()
}

func (c *runtimeContainer) imageRemove(ctx context.Context, a *Action) error {
	if crt, ok := c.crt.(driver.ContainerImageBuilder); ok {
		_, err := crt.ImageRemove(ctx, a.RuntimeDef().Container.Image, driver.ImageRemoveOptions{
			Force:         true,
			PruneChildren: false,
		})
		return err
	}

	return nil
}

func (c *runtimeContainer) isRebuildRequired(bi *driver.BuildDefinition) (bool, error) {
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

func (c *runtimeContainer) imageEnsure(ctx context.Context, a *Action) error {
	crt, ok := c.crt.(driver.ContainerImageBuilder)
	if !ok {
		return nil
	}
	streams := a.Input().Streams()
	image := a.RuntimeDef().Container.Image
	// Prepend action to have the top priority in image build resolution.
	r := ChainImageBuildResolver{append(ChainImageBuildResolver{a}, c.imgres...)}

	buildInfo := r.ImageBuildInfo(image)
	forceRebuild, err := c.isRebuildRequired(buildInfo)
	if err != nil {
		return err
	}

	status, err := crt.ImageEnsure(ctx, driver.ImageOptions{
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
	case driver.ImageExists:
		log.Debug("image exists locally")
	case driver.ImagePull:
		if status.Progress == nil {
			break
		}
		defer func() {
			_ = status.Progress.Close()
		}()
		launchr.Term().Printfln("Image %q doesn't exist locally, pulling from the registry...", image)
		log.Info("image doesn't exist locally, pulling from the registry")
		// Output docker status only in Debug.
		err = driver.DockerDisplayJSONMessages(status.Progress, streams)
		if err != nil {
			launchr.Term().Error().Println("Error occurred while pulling the image %q", image)
			log.Error("error while pulling the image", "error", err)
		}
	case driver.ImageBuild:
		if status.Progress == nil {
			break
		}
		defer func() {
			_ = status.Progress.Close()
		}()
		launchr.Term().Printfln("Image %q doesn't exist locally, building...", image)
		log.Info("image doesn't exist locally, building the image")
		// Output docker status only in Debug.
		err = driver.DockerDisplayJSONMessages(status.Progress, streams)
		if err != nil {
			launchr.Term().Error().Println("Error occurred while building the image %q", image)
			log.Error("error while building the image", "error", err)
		}
	}

	return err
}

func (c *runtimeContainer) containerCreate(ctx context.Context, a *Action, opts *driver.ContainerCreateOptions) (string, error) {
	var err error
	// Sync to disk virtual actions so the data is available in run.
	if err = a.syncToDisk(); err != nil {
		return "", err
	}
	if err = c.imageEnsure(ctx, a); err != nil {
		return "", err
	}

	// Create a container
	runDef := a.RuntimeDef()

	// Override Cmd with exec command.
	if c.exec {
		runDef.Container.Command = a.Input().ArgsPositional()
	}

	createOpts := driver.ContainerCreateOptions{
		ContainerName: opts.ContainerName,
		Image:         runDef.Container.Image,
		Cmd:           runDef.Container.Command,
		WorkingDir:    containerHostMount,
		NetworkMode:   driver.NetworkModeHost,
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
			launchr.MustAbs(a.WorkDir()) + ":" + containerHostMount + flags,
			launchr.MustAbs(a.Dir()) + ":" + containerActionMount + flags,
		}
	}
	cid, err := c.crt.ContainerCreate(ctx, createOpts)
	if err != nil {
		return "", err
	}

	return cid, nil
}

// copyDirToContainer copies dir content to a container.
// Helpful to have the same owner in the destination directory.
func (c *runtimeContainer) copyDirToContainer(ctx context.Context, cid, srcPath, dstPath string) error {
	return c.copyToContainer(ctx, cid, srcPath, filepath.Dir(dstPath), filepath.Base(dstPath))
}

// copyToContainer copies dir/file to a container. Directory will be copied as a subdirectory.
func (c *runtimeContainer) copyToContainer(ctx context.Context, cid, srcPath, dstPath, rebaseName string) error {
	// Prepare destination copy info by stat-ing the container path.
	dstStat, err := c.crt.ContainerStatPath(ctx, cid, dstPath)
	if err != nil {
		return err
	}

	arch, err := archive.Tar(
		archive.CopyInfo{
			Path:       srcPath,
			RebaseName: rebaseName,
		},
		archive.CopyInfo{
			Path:   dstPath,
			Exists: true,
			IsDir:  dstStat.Mode.IsDir(),
		},
		nil,
	)
	if err != nil {
		return err
	}
	defer arch.Close()

	dstDir := dstPath
	if !dstStat.Mode.IsDir() {
		dstDir = filepath.Base(dstPath)
	}
	options := driver.CopyToContainerOptions{
		AllowOverwriteDirWithFile: false,
		CopyUIDGID:                false,
	}
	return c.crt.CopyToContainer(ctx, cid, dstDir, arch, options)
}

func (c *runtimeContainer) copyFromContainer(ctx context.Context, cid, srcPath, dstPath, rebaseName string) (err error) {
	content, stat, err := c.crt.CopyFromContainer(ctx, cid, srcPath)
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

	return archive.Untar(content, dstPath, &archive.TarOptions{SrcInfo: srcInfo})
}

func (c *runtimeContainer) containerWait(ctx context.Context, cid string, opts *driver.ContainerCreateOptions) <-chan int {
	log := c.log()
	// Wait for the container to stop or catch error.
	waitCond := driver.WaitConditionNextExit
	if opts.AutoRemove {
		waitCond = driver.WaitConditionRemoved
	}
	resCh, errCh := c.crt.ContainerWait(ctx, cid, driver.ContainerWaitOptions{Condition: waitCond})
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

func (c *runtimeContainer) attachContainer(ctx context.Context, streams launchr.Streams, cid string, opts *driver.ContainerCreateOptions) (io.Closer, <-chan error, error) {
	cio, errAttach := c.crt.ContainerAttach(ctx, cid, driver.ContainerAttachOptions{
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

func (c *runtimeContainer) isSELinuxEnabled(ctx context.Context) bool {
	// First, we check if it's enabled at the OS level, then if it's enabled in the container runner.
	// If the feature is not enabled in the runner environment,
	// containers will bypass SELinux and will function as if SELinux is disabled in the OS.
	d, ok := c.crt.(driver.ContainerRunnerSELinux)
	return ok && launchr.IsSELinuxEnabled() && d.IsSELinuxSupported(ctx)
}
