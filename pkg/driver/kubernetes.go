package driver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"time"

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

const k8sMainPodContainer = "supervisor"
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

	// Create the pod definition.
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			HostAliases:   hostAliases,
			Hostname:      opts.Hostname,
			RestartPolicy: corev1.RestartPolicyNever,
			Volumes:       volumes,
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
	_, err := k.clientset.CoreV1().
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
	return cid, err
}

func (k *k8sRuntime) ContainerStart(ctx context.Context, cid string, opts ContainerDefinition) (<-chan int, *ContainerInOut, error) {
	// Create an ephemeral container to run.
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

	ephemeralContainer := corev1.EphemeralContainer{
		EphemeralContainerCommon: corev1.EphemeralContainerCommon{
			Name:  containerName,
			Image: opts.Image,
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
				return containerStatus.State.Running != nil, nil
			}
		}
		return false, nil
	})
}
