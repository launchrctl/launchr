package driver

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/httpstream"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
)

func k8sGetConfig() (*restclient.Config, error) {
	// Try to use in-cluster config
	config, err := restclient.InClusterConfig()
	if err != nil {
		// Fall back to kubeconfig
		kubeconfig := os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			kubeconfig = filepath.Join(os.Getenv("HOME"), ".kube", "config")
		}

		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to get Kubernetes config: %w", err)
		}
	}

	return config, nil
}

func k8sCreateExecutor(url *url.URL, config *restclient.Config) (remotecommand.Executor, error) {
	executor, err := remotecommand.NewSPDYExecutor(config, "POST", url)
	if err != nil {
		return nil, err
	}
	// Fallback executor is default, unless feature flag is explicitly disabled.
	if k8sUseWebsocket {
		// WebSocketExecutor must be "GET" method as described in RFC 6455 Sec. 4.1 (page 17).
		websocketExec, err := remotecommand.NewWebSocketExecutor(config, "GET", url.String())
		if err != nil {
			return nil, err
		}
		executor, err = remotecommand.NewFallbackExecutor(websocketExec, executor, func(err error) bool {
			return httpstream.IsUpgradeFailure(err) || httpstream.IsHTTPSProxyError(err)
		})
		if err != nil {
			return nil, err
		}
	}
	return executor, nil
}

func k8sCreateContainerID(namespace, podName, containerName string) string {
	return namespace + "/" + podName + "/" + containerName
}

func k8sPodBuildContainerID(cid string) string {
	namespace, podName, _ := k8sParseContainerID(cid)
	return k8sCreateContainerID(namespace, podName, k8sBuildPodContainer)
}

func k8sPodMainContainerID(cid string) string {
	namespace, podName, _ := k8sParseContainerID(cid)
	return k8sCreateContainerID(namespace, podName, k8sMainPodContainer)
}

func k8sParseContainerID(cid string) (string, string, string) {
	parts := strings.SplitN(cid, "/", 3)
	return parts[0], parts[1], parts[2]
}

func k8sVolumesAndMounts(opts ContainerDefinition) ([]corev1.Volume, []corev1.VolumeMount) {
	// Prepare volumes.
	containerName := opts.ContainerName
	volumes := make([]corev1.Volume, len(opts.Volumes))
	mounts := make([]corev1.VolumeMount, len(opts.Volumes))
	for i := 0; i < len(volumes); i++ {
		name := opts.Volumes[i].Name
		if name == "" {
			name = containerName + "-" + strconv.Itoa(i)
		}
		mounts[i] = corev1.VolumeMount{
			Name:      name,
			MountPath: opts.Volumes[i].MountPath,
		}
		volumes[i] = corev1.Volume{
			Name: name,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		}
	}
	return volumes, mounts
}

func k8sEnvVars(opts ContainerDefinition) []corev1.EnvVar {
	envVars := make([]corev1.EnvVar, len(opts.Env))
	for i := 0; i < len(envVars); i++ {
		parts := strings.SplitN(opts.Env[i], "=", 2)
		envVars[i] = corev1.EnvVar{Name: parts[0], Value: parts[1]}
	}
	return envVars
}

func k8sHostAliases(opts ContainerDefinition) []corev1.HostAlias {
	hostAliases := make([]corev1.HostAlias, len(opts.ExtraHosts))
	for i := 0; i < len(hostAliases); i++ {
		parts := strings.SplitN(opts.ExtraHosts[i], ":", 2)
		hostAliases[i] = corev1.HostAlias{
			IP:        parts[1],
			Hostnames: parts[0:1],
		}
	}
	return hostAliases
}

type k8sResizeQueue chan *remotecommand.TerminalSize

func (q k8sResizeQueue) Next() *remotecommand.TerminalSize { return <-q }

// There is a strange bug that tar keeps streaming zero bytes after EOF.
// Maybe because the consumer (stdout in k8s streamer) is not handling it.
// The command execution ends because of `pipeReader.Close()`
// and it gives an error because it writes in a closed pipe.
// We prevent it by sending a special error.
type k8sTarPipeReader struct{ *io.PipeReader }

func (r *k8sTarPipeReader) Close() error { return r.CloseWithError(errK8sStopTarPipeWrite) }

type k8sStreams struct {
	in   io.Reader
	out  io.Writer
	err  io.Writer
	opts ContainerStreamsOptions
	tty  remotecommand.TerminalSizeQueue
}

func (s k8sStreams) streamOptions() remotecommand.StreamOptions {
	var stdout, stderr io.Writer
	var stdin io.Reader
	if s.opts.Stdin {
		stdin = s.in
	}
	if s.opts.Stdout {
		stdout = s.out
	}
	// Do not set stderr because both stdout and stderr go over stdout when tty is set.
	if s.opts.Stderr && !s.opts.TTY {
		stderr = s.err
	}

	return remotecommand.StreamOptions{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
		Tty:    s.opts.TTY,

		TerminalSizeQueue: s.tty,
	}
}

// parseStatOutput parses the formatted output from the stat command
func parseStatOutput(output string, path string) ContainerPathStat {
	// Split the stat output by pipe character
	// Format: name|size|mode_hex|mtime_unix|linktarget
	const (
		idxName = iota
		idxSize
		idxMode
		idxTime
		idxLink
	)
	output = strings.TrimSpace(output)
	parts := strings.Split(output, "|")
	if len(parts) < 5 {
		panic(fmt.Sprintf("invalid stat output format: %s, path %s", output, path))
	}

	// Parse size
	size, err := strconv.ParseInt(parts[idxSize], 10, 64)
	if err != nil {
		panic(fmt.Errorf("failed to parse file size: %w", err))
	}

	// Parse mode (hex format from stat -c %f)
	modeHex, err := strconv.ParseUint(parts[idxMode], 16, 32)
	if err != nil {
		panic(fmt.Errorf("failed to parse file mode: %w", err))
	}
	mode := fillFileStatFromSys(uint32(modeHex)) //nolint:gosec // G115: overflow should be ok

	// Parse modification time (unix timestamp)
	mtimeUnix, err := strconv.ParseInt(parts[idxTime], 10, 64)
	if err != nil {
		panic(fmt.Errorf("failed to parse modification time: %w", err))
	}
	mtime := time.Unix(mtimeUnix, 0)

	// Return the PathStat structure
	return ContainerPathStat{
		Name:       parts[idxName],
		Size:       size,
		Mode:       mode,
		Mtime:      mtime,
		LinkTarget: parts[idxLink],
	}
}

// fillFileStatFromSys parses linux stat output.
// Based on the linux version of [os.fillFileStatFromSys].
func fillFileStatFromSys(modeHex uint32) os.FileMode {
	//nolint // Preserve the same names as in [syscall].
	const (
		S_IFBLK  = 0x6000
		S_IFCHR  = 0x2000
		S_IFDIR  = 0x4000
		S_IFIFO  = 0x1000
		S_IFLNK  = 0xa000
		S_IFMT   = 0xf000
		S_IFREG  = 0x8000
		S_IFSOCK = 0xc000
		S_ISGID  = 0x400
		S_ISUID  = 0x800
		S_ISVTX  = 0x200
	)

	mode := os.FileMode(modeHex)
	mode = mode & os.ModePerm

	switch modeHex & S_IFMT {
	case S_IFBLK:
		mode |= os.ModeDevice
	case S_IFCHR:
		mode |= os.ModeDevice | os.ModeCharDevice
	case S_IFDIR:
		mode |= os.ModeDir
	case S_IFIFO:
		mode |= os.ModeNamedPipe
	case S_IFLNK:
		mode |= os.ModeSymlink
	case S_IFREG:
		// nothing to do
	case S_IFSOCK:
		mode |= os.ModeSocket
	}
	if mode&S_ISGID != 0 {
		mode |= os.ModeSetgid
	}
	if mode&S_ISUID != 0 {
		mode |= os.ModeSetuid
	}
	if mode&S_ISVTX != 0 {
		mode |= os.ModeSticky
	}
	return mode
}

func ensureBuildFile(file string) string {
	if file != "" {
		return file
	}

	return "Dockerfile"
}
