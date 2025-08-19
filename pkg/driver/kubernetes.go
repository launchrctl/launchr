package driver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/util/exec"

	"github.com/launchrctl/launchr/internal/launchr"
)

// KubernetesRegistry defines a registry type for a kubernetes container.
type KubernetesRegistry string

// String returns the string representation of the registry type.
func (k KubernetesRegistry) String() string {
	return string(k)
}

// RegistryInternal defines an internal registry type
const RegistryInternal KubernetesRegistry = "internal"

// RegistryRemote defines a remote registry type
const RegistryRemote KubernetesRegistry = "remote"

// RegistryNone defines no registry type
const RegistryNone KubernetesRegistry = "none"

// RegistryFromString creates a [KubernetesRegistry] with enum validation.
func RegistryFromString(t string) KubernetesRegistry {
	if t == "" {
		return RegistryNone
	}
	switch KubernetesRegistry(t) {
	case RegistryInternal, RegistryRemote, RegistryNone:
		return KubernetesRegistry(t)
	default:
		return RegistryNone
	}
}

const k8sMainPodContainer = "supervisor"
const k8sBuildPodContainer = "image-builder"

// Image build and image consist of multiple layers, artifacts, intermediate containers, and buildah needs temporary
// space to store these during the build process. 2Gi provides enough space for most typical application images.
const k8sBuildahStorageLimit = "2Gi"
const k8sUseWebsocket = true
const k8sStatPathScript = `
FILE="%s"
if [ -e "$FILE" ]; then
	# Get file stats
	STAT=$(stat -c "%%n|%%s|%%f|%%Y" "$FILE")
	# Check if it's a symlink and get target if it is
	if [ -L "$FILE" ]; then
		TARGET=$(readlink -n "$FILE")
		echo "$STAT|$TARGET"
	else
		echo "$STAT|"
	fi
	exit 0
else
	echo "File not found: $FILE" >&2
	exit 1
fi
`
const k8sWaitAttachScript = `
# Wait for signal USR1 to break loop
signal_received=0
handle_signal() {
    signal_received=1
}
trap 'handle_signal' USR1

# Wait until signal_received becomes 1
while [ "$signal_received" -eq 0 ]; do
    sleep 1
done

exec "$@"
`

var errK8sStopTarPipeWrite = errors.New("k8s: break tar pipe write")

func init() {
	// Override k8s logger.
	runtime.ErrorHandlers = []runtime.ErrorHandler{
		k8sLogError,
	}
}

func k8sLogError(_ context.Context, err error, msg string, keysAndValues ...interface{}) {
	if err == errK8sStopTarPipeWrite {
		return
	}
	launchr.Log().
		With(keysAndValues...).
		Debug("unhandled error in kubernetes runtime", "error", err, "msg", msg)
}

type k8sRuntime struct {
	config    *restclient.Config
	clientset *kubernetes.Clientset
}

// NewKubernetesRuntime creates a kubernetes container runtime.
func NewKubernetesRuntime() (ContainerRunner, error) {
	// Get Kubernetes config
	config, err := k8sGetConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get Kubernetes config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	return &k8sRuntime{
		config:    config,
		clientset: clientset,
	}, nil
}

func (k *k8sRuntime) Info(_ context.Context) (SystemInfo, error) {
	return SystemInfo{
		// Kubernetes is always a remote environment.
		Remote: true,
	}, nil
}

func (k *k8sRuntime) CopyToContainer(ctx context.Context, cid string, path string, content io.Reader, opts CopyToContainerOptions) error {
	// Create the command to extract the tar
	var cmdArr []string
	if opts.CopyUIDGID {
		cmdArr = []string{"tar", "-xmf", "-"}
	} else {
		cmdArr = []string{"tar", "--no-same-permissions", "--no-same-owner", "-xmf", "-"}
	}
	cmdArr = append(cmdArr, "-C", path)

	// Execute the command in the container, streaming in the tar file
	return k.containerExec(ctx, k8sPodMainContainerID(cid), cmdArr, k8sStreams{
		in: content,
		opts: ContainerStreamsOptions{
			Stdin: true,
		},
	})
}

func (k *k8sRuntime) CopyFromContainer(ctx context.Context, cid, srcPath string) (io.ReadCloser, ContainerPathStat, error) {
	// Test path info.
	pathStat, err := k.ContainerStatPath(ctx, cid, srcPath)
	if err != nil {
		return nil, ContainerPathStat{}, err
	}

	// Execute the command in the container, streaming in the tar file
	cmdArr := []string{"tar", "cf", "-", srcPath}

	// Pipe tar data to return.
	pipeReader, outStream := io.Pipe()

	// Start streaming from the container.
	go func() {
		defer outStream.Close()
		// We need to attach stdout to wait for result.
		err = k.containerExec(ctx, k8sPodMainContainerID(cid), cmdArr, k8sStreams{
			out: outStream,
			opts: ContainerStreamsOptions{
				Stdout: true,
			},
		})

		if err != nil {
			launchr.Log().Debug("failed to copy from container", "cid", cid, "srcPath", srcPath, "err", err)
		}
	}()

	return &k8sTarPipeReader{pipeReader}, pathStat, nil
}

func (k *k8sRuntime) ContainerStatPath(ctx context.Context, cid string, path string) (ContainerPathStat, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	done := make(chan error)
	statCmd := []string{"sh", "-c", fmt.Sprintf(k8sStatPathScript, path)}

	// Capture output
	var stdout bytes.Buffer
	go func() {
		done <- k.containerExec(ctx, k8sPodMainContainerID(cid), statCmd, k8sStreams{
			out: &stdout,
			opts: ContainerStreamsOptions{
				Stdout: true,
			},
		})
	}()

	select {
	case <-ctx.Done():
		return ContainerPathStat{}, ctx.Err()
	case err := <-done:
		if err != nil {
			return ContainerPathStat{}, err
		}
		return parseStatOutput(stdout.String(), path), nil
	}

}

func (k *k8sRuntime) ContainerList(_ context.Context, _ ContainerListOptions) []ContainerListResult {
	return nil
}

func (k *k8sRuntime) ContainerCreate(ctx context.Context, opts ContainerDefinition) (string, error) {
	// Generate a unique pod name
	namespace := "default"
	podName := opts.ContainerName
	containerName := podName

	cid := k8sCreateContainerID(namespace, podName, containerName)

	// Prepare environment variables, host aliases and volumes.
	hostAliases := k8sHostAliases(opts)
	volumes, mounts := k8sVolumesAndMounts(opts)

	sidecars, volumes, mounts, err := k.prepareSidecarContainers(volumes, mounts, opts)
	if err != nil {
		return "", err
	}

	// Create the pod definition.
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			HostAliases:    hostAliases,
			Hostname:       opts.Hostname,
			HostNetwork:    true,
			RestartPolicy:  corev1.RestartPolicyNever,
			Volumes:        volumes,
			InitContainers: sidecars,
			Containers: []corev1.Container{
				{
					Name:         k8sMainPodContainer,
					Image:        "alpine:latest",
					VolumeMounts: mounts,
					Command:      []string{"sleep"},
					Args:         []string{"infinity"},
				},
			},
		},
	}

	// Create the pod
	launchr.Log().Debug("creating pod", "namespace", namespace, "pod", podName)
	_, err = k.clientset.CoreV1().
		Pods(namespace).
		Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to create pod: %w", err)
	}
	// Wait for pod to be running
	launchr.Log().Debug("waiting for pod to start running", "namespace", namespace, "pod", podName)
	err = wait.PollUntilContextTimeout(ctx, time.Millisecond*300, time.Second*30, true, func(ctx context.Context) (bool, error) {
		pod, err := k.clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return false, nil // Ignore errors and keep trying
		}
		return pod.Status.Phase == corev1.PodRunning, nil
	})

	if err != nil {
		return "", fmt.Errorf("error waiting for pod to run: %w", err)
	}

	launchr.Log().Debug("pod is running", "namespace", namespace, "pod", podName)
	return cid, nil
}

func (k *k8sRuntime) ContainerStart(ctx context.Context, cid string, opts ContainerDefinition) (<-chan int, *ContainerInOut, error) {
	err := k.addEphemeralContainer(ctx, cid, opts)
	if err != nil {
		return nil, nil, err
	}

	statusCh := make(chan int)

	// Prepare container io to handle tty.
	// Create pipes for stdin, stdout, and stderr.
	stdinReader, stdinWriter := io.Pipe()
	stdoutReader, stdoutWriter := io.Pipe()
	stderrReader, stderrWriter := io.Pipe()

	cio := &ContainerInOut{
		In:   stdinWriter,
		Out:  stdoutReader,
		Err:  stderrReader,
		Opts: opts.Streams,
	}

	var resizeCh k8sResizeQueue
	if opts.Streams.TTY {
		resizeCh = make(k8sResizeQueue, 1)
		cio.TtyMonitor = NewTtySizeMonitor(func(_ context.Context, ropts terminalSize) error {
			resizeCh <- &remotecommand.TerminalSize{
				Width:  uint16(ropts.Width),  //nolint:gosec // G115: overflow should be ok
				Height: uint16(ropts.Height), //nolint:gosec // G115: overflow should be ok
			}
			return nil
		})
	}

	// Stream container exec.
	go func() {
		defer close(statusCh)
		defer close(resizeCh)
		// Close writers when the execution finishes.
		defer stdoutWriter.Close()
		defer stderrWriter.Close()

		// Send a special signal to start the script after attach.
		err := k.ContainerKill(ctx, cid, "USR1")
		if err != nil {
			launchr.Log().Error("error container start", "error", err, "cid", cid)
			if exitErr, ok := err.(exec.CodeExitError); ok {
				statusCh <- exitErr.ExitStatus()
			} else {
				statusCh <- 130
			}
			return
		}

		// Wait io streaming to fully finish.
		err = k.containerAttach(ctx, cid, k8sStreams{
			in:   stdinReader,
			out:  stdoutWriter,
			err:  stderrWriter,
			opts: opts.Streams,
			tty:  resizeCh,
		})
		if err != nil {
			launchr.Log().Error("error container attach", "error", err, "cid", cid)
			if exitErr, ok := err.(exec.CodeExitError); ok {
				statusCh <- exitErr.ExitStatus()
			} else {
				statusCh <- 130
			}
		} else {
			statusCh <- 0
		}
	}()

	return statusCh, cio, nil
}

func (k *k8sRuntime) ContainerStop(ctx context.Context, cid string, opts ContainerStopOptions) error {
	timeout := 10 * time.Second
	if opts.Timeout != nil {
		timeout = *opts.Timeout
	}
	// Try to shut down gracefully within given timeout.
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- k.ContainerKill(ctx, cid, "TERM")
	}()

	var err error
	select {
	case err = <-errCh:
	case <-ctx.Done():
		err = ctx.Err()
	}
	// If failed to shut down, kill the process.
	if err != nil {
		return k.ContainerKill(ctx, cid, "KILL")
	}
	return nil
}

func (k *k8sRuntime) ContainerKill(ctx context.Context, cid, signal string) error {
	killCmd := []string{
		"/bin/sh", "-c",
		fmt.Sprintf("kill -%s 1", signal),
	}
	var stdout bytes.Buffer
	err := k.containerExec(ctx, cid, killCmd, k8sStreams{
		out: &stdout,
		opts: ContainerStreamsOptions{
			Stdout: true,
		},
	})
	if err != nil {
		return fmt.Errorf("error container kill: %w, message: %s", err, stdout.String())
	}
	return err
}

func (k *k8sRuntime) ContainerRemove(ctx context.Context, cid string) error {
	namespace, podName, _ := k8sParseContainerID(cid)
	deletePolicy := metav1.DeletePropagationForeground
	execOpts := metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}
	err := k.clientset.CoreV1().Pods(namespace).Delete(ctx, podName, execOpts)
	return err
}

func (k *k8sRuntime) Close() error {
	// Normally all requests are closed immediately.
	return nil
}

func (k *k8sRuntime) containerExec(ctx context.Context, cid string, cmd []string, streams k8sStreams) error {
	namespace, podName, containerName := k8sParseContainerID(cid)

	// Create the execution request
	req := k.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec")

	// Set up the exec options
	execOptions := &corev1.PodExecOptions{
		Container: containerName,
		Command:   cmd,
		Stdin:     streams.opts.Stdin,
		Stdout:    streams.opts.Stdout,
		Stderr:    streams.opts.Stderr,
		TTY:       streams.opts.TTY,
	}

	// Add the options to the request
	req.VersionedParams(execOptions, scheme.ParameterCodec)

	// Create the executor
	executor, err := k8sCreateExecutor(req.URL(), k.config)
	if err != nil {
		return fmt.Errorf("error creating executor: %w", err)
	}

	// Start the exec session
	return executor.StreamWithContext(ctx, streams.streamOptions())
}

func (k *k8sRuntime) containerAttach(ctx context.Context, cid string, streams k8sStreams) error {
	namespace, podName, containerName := k8sParseContainerID(cid)

	// Attach to the pod.
	req := k.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("attach")

	req.VersionedParams(&corev1.PodAttachOptions{
		Container: containerName,
		Stdin:     streams.opts.Stdin,
		Stdout:    streams.opts.Stdout,
		Stderr:    streams.opts.Stderr,
		TTY:       streams.opts.TTY,
	}, scheme.ParameterCodec)

	executor, err := k8sCreateExecutor(req.URL(), k.config)
	if err != nil {
		return fmt.Errorf("error creating executor: %w", err)
	}

	return executor.StreamWithContext(ctx, streams.streamOptions())
}

func (k *k8sRuntime) addEphemeralContainer(ctx context.Context, cid string, opts ContainerDefinition) error {
	namespace, podName, containerName := k8sParseContainerID(cid)
	_, mounts := k8sVolumesAndMounts(opts)

	cmd := slices.Concat(opts.Entrypoint, opts.Command)

	imageName := opts.Image
	pullPolicy := corev1.PullIfNotPresent
	if opts.ImageOptions.Build != nil && opts.ImageOptions.RegistryType != RegistryNone {
		// always pull to skip the kubernetes internal cache.
		pullPolicy = corev1.PullAlways
		imageName = fmt.Sprintf("%s/%s", opts.ImageOptions.RegistryURL, opts.Image)
	}

	ephemeralContainer := corev1.EphemeralContainer{
		EphemeralContainerCommon: corev1.EphemeralContainerCommon{
			Name:            containerName,
			Image:           imageName,
			ImagePullPolicy: pullPolicy,
			// Wrap the command into a script that will wait until a special signal USR1.
			// We do that to not miss any output before the attach. See ContainerStart.
			Command:      []string{"/bin/sh", "-c", k8sWaitAttachScript, "--"},
			Args:         cmd,
			WorkingDir:   opts.WorkingDir,
			VolumeMounts: mounts,
			Env:          k8sEnvVars(opts),
			TTY:          opts.Streams.TTY,
			Stdin:        opts.Streams.Stdin,
		},
	}

	// Create patch payload for ephemeral containers
	type patchSpec struct {
		Spec struct {
			EphemeralContainers []corev1.EphemeralContainer `json:"ephemeralContainers"`
		} `json:"spec"`
	}

	payload := patchSpec{}
	payload.Spec.EphemeralContainers = []corev1.EphemeralContainer{ephemeralContainer}

	// Convert to JSON
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal patch payload: %w", err)
	}

	// Apply the patch - use special ephemeralcontainers subresource
	_, err = k.clientset.CoreV1().Pods(namespace).Patch(
		ctx,
		podName,
		types.StrategicMergePatchType,
		payloadBytes,
		metav1.PatchOptions{},
		"ephemeralcontainers",
	)
	if err != nil {
		return fmt.Errorf("failed to patch ephemeral container to pod: %w", err)
	}

	// Wait until it's created.
	return wait.PollUntilContextTimeout(ctx, time.Millisecond*300, time.Second*30, true, func(ctx context.Context) (bool, error) {
		pod, err := k.clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}

		// Check ephemeral container status
		for _, containerStatus := range pod.Status.EphemeralContainerStatuses {
			if containerStatus.Name == containerName {
				if containerStatus.State.Terminated != nil {
					return true, fmt.Errorf("ephemeral container %s has terminated with exit code %d", containerName, containerStatus.State.Terminated.ExitCode)
				}
				waitStatus := containerStatus.State.Waiting
				if waitStatus != nil && strings.HasPrefix(waitStatus.Reason, "Err") {
					return true, fmt.Errorf("failed to create ephemeral container (%s): %s", waitStatus.Reason, waitStatus.Message)
				}
				return containerStatus.State.Running != nil, nil
			}
		}
		return false, nil
	})
}

func (k *k8sRuntime) prepareSidecarContainers(volumes []corev1.Volume, mounts []corev1.VolumeMount, opts ContainerDefinition) ([]corev1.Container, []corev1.Volume, []corev1.VolumeMount, error) {
	regType := opts.ImageOptions.RegistryType
	var containers []corev1.Container

	if regType == RegistryNone {
		return containers, volumes, mounts, nil
	}

	if regType == RegistryInternal {
		// @todo would be great to implement internal type which includes registry as sidecar and builds everything
		//    inside pod. Init containers don't share network between main container and sidecar containers,
		//    so probably we should combine main and sidecar containers together.
		panic("registry internal is not supported yet")
	}

	buildahVolumes := []corev1.Volume{
		{
			Name: "buildah-storage",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					SizeLimit: &[]resource.Quantity{resource.MustParse(k8sBuildahStorageLimit)}[0],
				},
			},
		},
	}
	buildahMounts := []corev1.VolumeMount{
		{
			Name:      "buildah-storage",
			MountPath: "/var/lib/containers",
		},
	}

	volumes = append(volumes, buildahVolumes...)
	mounts = append(mounts, buildahMounts...)

	securityContext := &corev1.SecurityContext{
		Privileged: &[]bool{true}[0],
		RunAsUser:  &[]int64{0}[0],
	}
	sidecarPolicy := corev1.ContainerRestartPolicyAlways

	buildahContainer := corev1.Container{
		Name:            k8sBuildPodContainer,
		Image:           "quay.io/buildah/stable:latest", // @todo change latest to a specific version ?
		SecurityContext: securityContext,
		RestartPolicy:   &sidecarPolicy,
		Command:         []string{"/bin/bash"},
		Args: []string{
			"-c",
			k.prepareBuildahInitScript(opts.ImageOptions),
		},
		VolumeMounts: mounts,
		Env: []corev1.EnvVar{
			{
				Name:  "STORAGE_DRIVER",
				Value: "vfs",
			},
			{
				Name:  "BUILDAH_ISOLATION",
				Value: "chroot",
			},
		},
	}

	containers = append(containers, buildahContainer)

	return containers, volumes, mounts, nil
}

func (k *k8sRuntime) ImageEnsure(ctx context.Context, opts ImageOptions) (*ImageStatusResponse, error) {
	if opts.RegistryType == RegistryNone || opts.Build == nil {
		return &ImageStatusResponse{Status: ImagePull}, nil
	}

	exists, err := k.doImageEnsure(ctx, opts)
	if err != nil {
		return &ImageStatusResponse{Status: ImageUnexpectedError}, err
	}

	if exists && !opts.ForceRebuild && !opts.NoCache {
		return &ImageStatusResponse{Status: ImagePull}, nil
	}

	err = k.doImageBuild(ctx, opts)
	if err != nil {
		return &ImageStatusResponse{Status: ImageUnexpectedError}, err
	}

	return &ImageStatusResponse{Status: ImageBuild}, nil
}

func (k *k8sRuntime) doImageEnsure(ctx context.Context, opts ImageOptions) (bool, error) {
	imageName, tag := parseImageName(opts.Name)
	imageURL := fmt.Sprintf("%s/v2/%s/manifests/%s", opts.RegistryURL, imageName, tag)
	imageCheckScript := fmt.Sprintf(buildahImageEnsureTemplate, imageURL)
	cmdArr := []string{
		"/bin/bash", "-c",
		imageCheckScript,
	}

	var stdout bytes.Buffer
	var exitCode int
	err := k.containerExec(ctx, opts.BuildContainerID, cmdArr, k8sStreams{
		out: &stdout,
		opts: ContainerStreamsOptions{
			Stdout: true,
		},
	})

	if err == nil {
		return true, nil
	}

	// Check if it's a CodeExitError that contains the exit status
	var exitErr exec.CodeExitError
	if errors.As(err, &exitErr) {
		exitCode = exitErr.ExitStatus()
	}

	// If the exit code is 2 (manually returned code), it means that the image does not exist.
	if exitCode == 2 {
		return false, nil
	}

	return false, fmt.Errorf("error container exec: %w, message: %s", err, stdout.String())
}

func (k *k8sRuntime) doImageBuild(ctx context.Context, opts ImageOptions) error {
	cmdArr := []string{
		"/bin/bash", "-c",
		k.prepareBuildahWorkScript(opts.Name, opts),
	}

	var stdout bytes.Buffer
	err := k.containerExec(ctx, opts.BuildContainerID, cmdArr, k8sStreams{
		out: &stdout,
		opts: ContainerStreamsOptions{
			Stdout: true,
		},
	})

	launchr.Log().Debug("build output: ", "output", stdout.String())
	if err != nil {
		return fmt.Errorf("error container exec: %w", err)
	}

	return nil
}

func (k *k8sRuntime) ImageRemove(ctx context.Context, image string, removeOpts ImageRemoveOptions) (*ImageRemoveResponse, error) {
	if removeOpts.RegistryType != RegistryRemote {
		return &ImageRemoveResponse{Status: ImageUnexpectedError}, nil
	}

	imageName, tag := parseImageName(image)
	type removeData struct {
		ImageName   string
		RegistryURL string
		Tag         string
	}
	data := &removeData{
		Tag:         tag,
		ImageName:   imageName,
		RegistryURL: removeOpts.RegistryURL,
	}

	script, err := executeTemplate(registryImageRemovalTemplate, data)
	if err != nil {
		panic(fmt.Errorf("failed to generate registry removal script: %s", err.Error()))
	}

	var stdout bytes.Buffer
	err = k.containerExec(ctx, removeOpts.BuildContainerID, []string{"/bin/bash", "-c", script}, k8sStreams{
		out:  &stdout,
		opts: ContainerStreamsOptions{Stdout: true},
	})

	launchr.Log().Info("registry removal output", "output", stdout.String())
	if err != nil {
		return &ImageRemoveResponse{Status: ImageUnexpectedError}, fmt.Errorf("failed to remove image from registry: %w", err)
	}

	return &ImageRemoveResponse{Status: ImageRemoved}, nil
}
