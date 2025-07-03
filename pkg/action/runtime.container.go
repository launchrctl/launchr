package action

import (
	"context"
	"errors"
	"fmt"
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
	containerFlagRemote       = "remote-runtime"
	containerFlagCopyBack     = "remote-copy-back"
	containerFlagRemoveImage  = "remove-image"
	containerFlagNoCache      = "no-cache"
	containerFlagRebuildImage = "rebuild-image"
	containerFlagEntrypoint   = "entrypoint"
	containerFlagExec         = "exec"
)

type runtimeContainer struct {
	WithLogger
	WithTerm
	WithFlagsGroup

	// crt is a container runtime.
	crt driver.ContainerRunner
	// rtype is a container runtime type string.
	rtype driver.Type
	// isRemoteRuntime checks if a container is run remotely.
	isRemoteRuntime bool

	// Container related functionality extenders
	// @todo migrate to events/hooks for loose coupling.
	imgres   ChainImageBuildResolver
	imgccres *ImageBuildCacheResolver
	nameprv  ContainerNameProvider

	// Runtime flags
	isSetRemote   bool
	copyBack      bool
	removeImg     bool
	noCache       bool
	rebuildImage  bool
	entrypoint    string
	entrypointSet bool
	exec          bool
	volumeFlags   string
}

// ContainerNameProvider provides an ability to generate a random container name
type ContainerNameProvider struct {
	Prefix       string
	RandomSuffix bool
}

// Get generates a new container name
func (p ContainerNameProvider) Get(name string) string {
	var rpl = strings.NewReplacer("_", "-", ":", "-", ".", "-")
	suffix := ""
	if p.RandomSuffix {
		suffix = "_" + launchr.GetRandomString(4)
	}

	return rpl.Replace(p.Prefix + name + suffix)
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
	rc := &runtimeContainer{
		rtype:   t,
		nameprv: ContainerNameProvider{Prefix: "launchr_", RandomSuffix: true},
	}

	rc.SetFlagsGroup(NewFlagsGroup(jsonschemaPropRuntime))
	return rc
}

func (c *runtimeContainer) Clone() Runtime {
	return NewContainerRuntime(c.rtype)
}

func (c *runtimeContainer) GetFlags() *FlagsGroup {
	flags := c.GetFlagsGroup()
	if len(flags.GetDefinitions()) == 0 {
		definitions := ParametersList{
			&DefParameter{
				Name:        containerFlagRemote,
				Title:       "Remote runtime",
				Description: "Forces the container runtime to be used as remote. Copies the working directory to a container volume. Local binds are not used.",
				Type:        jsonschema.Boolean,
				Default:     false,
			},
			&DefParameter{
				Name:        containerFlagCopyBack,
				Title:       "Remote copy back",
				Description: "Copies the working directory back from the container. Works only if the runtime is remote.",
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
				Name:        containerFlagRebuildImage,
				Title:       "Auto-rebuild image",
				Description: "Rebuild image if the action directory or the Dockerfile has changed",
				Type:        jsonschema.Boolean,
				Default:     true,
			},
			&DefParameter{
				Name:        containerFlagEntrypoint,
				Title:       "Image Entrypoint",
				Description: `Overwrite the default ENTRYPOINT of the image. Example: --entrypoint "/bin/sh"`,
				Type:        jsonschema.String,
				Default:     "",
			},
			&DefParameter{
				Name:        containerFlagExec,
				Title:       "Exec command",
				Description: "Overwrite the command of the action. Argument and options are not validated, sets container CMD directly. Example usage: --exec -- ls -lah",
				Type:        jsonschema.Boolean,
				Default:     false,
			},
		}

		flags.AddDefinitions(definitions)
	}

	return flags
}

func (c *runtimeContainer) ValidateInput(input *Input) error {
	err := c.flags.ValidateFlags(input.GroupFlags(c.flags.GetName()))
	if err != nil {
		return err
	}

	// early peak for an exec flag.
	exec := input.GetFlagInGroup(c.flags.GetName(), containerFlagExec)
	if exec != nil && exec.(bool) {
		// Mark input as validated because arguments are passed directly to exec.
		input.SetValidated(true)
	}

	return nil
}

func (c *runtimeContainer) SetFlags(input *Input) error {
	flags := input.GroupFlags(c.flags.GetName())

	if v, ok := flags[containerFlagRemote]; ok {
		c.isSetRemote = v.(bool)
	}

	if v, ok := flags[containerFlagCopyBack]; ok {
		c.copyBack = v.(bool)
	}

	if r, ok := flags[containerFlagRemoveImage]; ok {
		c.removeImg = r.(bool)
	}

	if nc, ok := flags[containerFlagNoCache]; ok {
		c.noCache = nc.(bool)
	}

	if rb, ok := flags[containerFlagRebuildImage]; ok {
		c.rebuildImage = rb.(bool)
	}

	if e, ok := flags[containerFlagEntrypoint]; ok && e != "" {
		c.entrypointSet = true
		c.entrypoint = e.(string)
	}

	if ex, ok := flags[containerFlagExec]; ok {
		c.exec = ex.(bool)
	}

	return nil
}

func (c *runtimeContainer) AddImageBuildResolver(r ImageBuildResolver) {
	c.imgres = append(c.imgres, r)
}
func (c *runtimeContainer) SetImageBuildCacheResolver(s *ImageBuildCacheResolver) { c.imgccres = s }
func (c *runtimeContainer) SetContainerNameProvider(p ContainerNameProvider)      { c.nameprv = p }

func (c *runtimeContainer) Init(ctx context.Context, _ *Action) (err error) {
	c.logWith = []any{"run_env", c.rtype}
	// Create the client.
	if c.crt == nil {
		c.crt, err = driver.New(c.rtype)
		if err != nil {
			return err
		}
	}
	// Check if the environment is remote.
	info, err := c.crt.Info(ctx)
	if err != nil {
		return err
	}
	c.isRemoteRuntime = info.Remote

	// Set mount flag for SELinux.
	if !c.isRemote() && c.isSELinuxEnabled(ctx) {
		// Check SELinux settings to allow reading the FS inside a container.
		// Use the lowercase z flag to allow concurrent actions access to the FS.
		c.volumeFlags += ":z"
		launchr.Term().Warning().Printfln(
			"SELinux is detected. The volumes will be mounted with the %q flags, which will relabel your files.\n"+
				"This process may take time or potentially break existing permissions.",
			c.volumeFlags,
		)
		c.Log().Warn("using selinux flags", "flags", c.volumeFlags)
	}

	return nil
}

func (c *runtimeContainer) Execute(ctx context.Context, a *Action) (err error) {
	ctx, cancelFn := context.WithCancel(ctx)
	defer cancelFn()

	// Prepare runtime variables.
	streams := a.Input().Streams()
	runDef := a.RuntimeDef()
	if runDef.Container == nil {
		return errors.New("action container configuration is not set, use different runtime")
	}
	log := c.LogWith("action_id", a.ID)
	log.Debug("starting execution of the action")

	// Generate a container name.
	name := c.nameprv.Get(a.ID)
	existing := c.crt.ContainerList(ctx, driver.ContainerListOptions{SearchName: name})
	if len(existing) > 0 {
		return fmt.Errorf("the action %q can't start, the container name is in use, please, try again", a.ID)
	}

	// Create a container.
	runConfig := c.createContainerDef(a, name)
	log = c.LogWith("image", runConfig.Image, "command", runConfig.Command, "entrypoint", runConfig.Entrypoint)
	log.Debug("creating a container for an action")
	cid, err := c.containerCreate(ctx, a, &runConfig)
	if err != nil {
		return fmt.Errorf("failed to create a container: %w", err)
	}
	if cid == "" {
		return errors.New("error on creating a container")
	}

	// Remove the container after finish.
	defer func() {
		log.Debug("remove container after run")
		errRm := c.crt.ContainerRemove(ctx, cid)
		if errRm != nil {
			log.Error("error on cleaning the running environment", "error", errRm)
		} else {
			log.Debug("container was successfully removed")
		}
	}()

	// Remove the used image if it was specified.
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

	log = c.LogWith("container_id", cid)
	log.Debug("successfully created a container for an action")

	// Copy working dirs to the container.
	err = c.copyAllToContainer(ctx, cid, a)
	if err != nil {
		return err
	}

	if !runConfig.Streams.TTY {
		log.Debug("watching container signals")
		sigc := launchr.NotifySignals()
		go launchr.HandleSignals(ctx, sigc, func(_ os.Signal, sig string) error {
			return c.crt.ContainerKill(ctx, cid, sig)
		})
		defer launchr.StopCatchSignals(sigc)
	}

	// Start the container
	log.Debug("starting container")
	statusCh, cio, err := c.crt.ContainerStart(ctx, cid, runConfig)
	if err != nil {
		log.Error("failed to start the container", "err", err)
		return err
	}

	// Stream container io and watch tty resize.
	go func() {
		if cio == nil {
			return
		}
		defer cio.Close()
		if runConfig.Streams.TTY {
			launchr.Log().Debug("watching TTY resize")
			cio.TtyMonitor.Start(ctx, streams)
		}
		errStream := cio.Stream(ctx, streams)
		if errStream != nil {
			launchr.Log().Error("error on streaming container io. The container may still run, waiting for it to finish", "error", err)
		}
	}()

	// Wait for the execution result code.
	log.Debug("waiting execution of the container")
	status := <-statusCh
	// @todo maybe we should note that SIG was sent to the container. Code 130 is sent on Ctlr+C.
	log.Info("action finished with the exit code", "exit_code", status)
	if status != 0 {
		err = launchr.NewExitError(status, fmt.Sprintf("action %q finished with exit code %d", a.ID, status))
	}

	// Copy back the result from the volume.
	errCp := c.copyAllFromContainer(ctx, cid, a)
	if err == nil {
		// If the run was successful, return a copy error to show that the result is not available.
		err = errCp
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

func (c *runtimeContainer) Close() error {
	if c.crt == nil {
		return nil
	}
	return c.crt.Close()
}

func (c *runtimeContainer) imageRemove(ctx context.Context, a *Action) error {
	if crt, ok := c.crt.(driver.ContainerImageBuilder); ok {
		_, err := crt.ImageRemove(ctx, a.RuntimeDef().Container.Image, driver.ImageRemoveOptions{
			Force: true,
		})
		return err
	}

	return nil
}

func (c *runtimeContainer) isRebuildRequired(bi *driver.BuildDefinition) (bool, error) {
	// @todo test image cache resolution somehow.
	if c.imgccres == nil || bi == nil || !c.rebuildImage {
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
		c.Log().Warn("failed to update actions.sum file", "error", errCache)
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

	log := c.Log()
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
		c.Term().Printfln("Image %q doesn't exist locally, pulling from the registry...", image)
		log.Info("image doesn't exist locally, pulling from the registry")
		// Output docker status only in Debug.
		err = status.Progress.Stream(streams.Out())
		if err != nil {
			c.Term().Error().Println("Error occurred while pulling the image %q", image)
			log.Error("error while pulling the image", "error", err)
		}
	case driver.ImageBuild:
		if status.Progress == nil {
			break
		}
		defer func() {
			_ = status.Progress.Close()
		}()
		c.Term().Printfln("Image %q doesn't exist locally, building...", image)
		log.Info("image doesn't exist locally, building the image")
		// Output docker status only in Debug.
		err = status.Progress.Stream(streams.Out())
		if err != nil {
			c.Term().Error().Println("Error occurred while building the image %q", image)
			log.Error("error while building the image", "error", err)
		}
	}

	return err
}

func (c *runtimeContainer) containerCreate(ctx context.Context, a *Action, createOpts *driver.ContainerDefinition) (string, error) {
	var err error
	if err = c.imageEnsure(ctx, a); err != nil {
		return "", err
	}

	cid, err := c.crt.ContainerCreate(ctx, *createOpts)
	if err != nil {
		return "", err
	}

	return cid, nil
}

func (c *runtimeContainer) createContainerDef(a *Action, cname string) driver.ContainerDefinition {
	// Create a container
	runDef := a.RuntimeDef()
	streams := a.Input().Streams()

	// Override an entrypoint if it was set in flags.
	var entrypoint []string
	if c.entrypointSet {
		entrypoint = []string{c.entrypoint}
	}

	// Override Command with exec command.
	cmd := runDef.Container.Command
	if c.exec {
		cmd = a.Input().ArgsPositional()
	}

	createOpts := driver.ContainerDefinition{
		ContainerName: cname,
		Image:         runDef.Container.Image,
		Command:       cmd,
		WorkingDir:    containerHostMount,
		ExtraHosts:    runDef.Container.ExtraHosts,
		Env:           runDef.Container.Env,
		User:          getCurrentUser(),
		Entrypoint:    entrypoint,
		Streams: driver.ContainerStreamsOptions{
			Stdin:  !streams.In().IsDiscard(),
			Stdout: !streams.Out().IsDiscard(),
			Stderr: !streams.Err().IsDiscard(),
			TTY:    streams.In().IsTerminal(),
		},
	}

	if c.isRemote() {
		// Use anonymous volumes to be removed after finish.
		createOpts.Volumes = containerAnonymousVolumes(
			containerHostMount,
			containerActionMount,
		)
	} else {
		createOpts.Binds = []string{
			launchr.MustAbs(a.WorkDir()) + ":" + containerHostMount + c.volumeFlags,
			launchr.MustAbs(a.Dir()) + ":" + containerActionMount + c.volumeFlags,
		}
	}
	return createOpts
}

func containerAnonymousVolumes(paths ...string) []driver.ContainerVolume {
	volumes := make([]driver.ContainerVolume, len(paths))
	for i := 0; i < len(paths); i++ {
		volumes[i] = driver.ContainerVolume{MountPath: paths[i]}
	}
	return volumes
}

func (c *runtimeContainer) copyAllToContainer(ctx context.Context, cid string, a *Action) (err error) {
	if !c.isRemote() {
		return nil
	}
	// @todo test somehow.
	launchr.Term().Info().Printfln(`Running in the remote environment. Copying the working directory and action directory inside the container.`)
	// Copy dir to a container to have the same owner in the destination directory.
	// Copying only the content of the dir will not override the parent dir ownership.
	err = c.copyToContainer(ctx, cid, a.WorkDir(), filepath.Dir(containerHostMount), filepath.Base(containerHostMount))
	if err != nil {
		return fmt.Errorf("failed to copy host directory to the container: %w", err)
	}
	err = c.copyToContainer(ctx, cid, a.Dir(), filepath.Dir(containerActionMount), filepath.Base(containerActionMount))
	if err != nil {
		return fmt.Errorf("failed to copy action directory to the container: %w", err)
	}
	return nil
}

func (c *runtimeContainer) copyAllFromContainer(ctx context.Context, cid string, a *Action) (err error) {
	if !c.isRemote() || !c.copyBack {
		return nil
	}
	// @todo it's a bad implementation considering consequential runs, need to find a better way to sync with remote.
	//   We may need to consider creating a session (by user command) before action run.
	//   After the session is created, create a named volume in docker or a pod+volume in k8s.
	//   All consequential actions will reuse the volume.
	//   After the session ends (by user command), copy all back.
	//   Delete volume or pod after finish.
	//   Or do not copy at all, define a session with prepare script that will prepare the environment.
	src := containerHostMount
	dst := a.WorkDir()

	launchr.Term().Info().Printfln(`Running in the remote environment and "--%s" is set. Copying back the result of the action run.`, containerFlagCopyBack)
	return c.copyFromContainer(ctx, cid, src, filepath.Dir(dst), filepath.Base(dst))
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

func (c *runtimeContainer) isSELinuxEnabled(ctx context.Context) bool {
	// First, we check if it's enabled at the OS level, then if it's enabled in the container runner.
	// If the feature is not enabled in the runner environment,
	// containers will bypass SELinux and will function as if SELinux is disabled in the OS.
	d, ok := c.crt.(driver.ContainerRunnerSELinux)
	return ok && launchr.IsSELinuxEnabled() && d.IsSELinuxSupported(ctx)
}

func (c *runtimeContainer) isRemote() bool {
	return c.isRemoteRuntime || c.isSetRemote
}
