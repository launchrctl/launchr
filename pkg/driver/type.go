// Package driver hold implementation for container runtimes.
package driver

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/launchrctl/launchr/internal/launchr"
)

// ContainerRunner defines common interface for container environments.
type ContainerRunner interface {
	Info(ctx context.Context) (SystemInfo, error)
	CopyToContainer(ctx context.Context, cid string, path string, content io.Reader, opts CopyToContainerOptions) error
	CopyFromContainer(ctx context.Context, cid, srcPath string) (io.ReadCloser, ContainerPathStat, error)
	ContainerStatPath(ctx context.Context, cid string, path string) (ContainerPathStat, error)
	ContainerList(ctx context.Context, opts ContainerListOptions) []ContainerListResult
	ContainerCreate(ctx context.Context, opts ContainerDefinition) (string, error)
	ContainerStart(ctx context.Context, cid string, opts ContainerDefinition) (<-chan int, *ContainerInOut, error)
	ContainerStop(ctx context.Context, cid string, opts ContainerStopOptions) error
	ContainerKill(ctx context.Context, cid, signal string) error
	ContainerRemove(ctx context.Context, cid string) error
	Close() error
}

// ContainerImageBuilder is an interface for container runtime to build images.
type ContainerImageBuilder interface {
	ContainerRunner
	ImageEnsure(ctx context.Context, opts ImageOptions) (*ImageStatusResponse, error)
	ImageRemove(ctx context.Context, image string, opts ImageRemoveOptions) (*ImageRemoveResponse, error)
}

// ContainerRunnerSELinux defines a container runner with SELinux support.
type ContainerRunnerSELinux interface {
	IsSELinuxSupported(ctx context.Context) bool
}

// BuildDefinition stores image build definition.
type BuildDefinition struct {
	Context   string             `yaml:"context"`
	Buildfile string             `yaml:"buildfile"`
	Args      map[string]*string `yaml:"args"`
	Tags      []string           `yaml:"tags"`
}

// ImageBuildInfo preprocesses build info to be ready for a container build.
func (b *BuildDefinition) ImageBuildInfo(name string, cwd string) *BuildDefinition {
	if b == nil {
		return nil
	}
	build := *b
	if !filepath.IsAbs(b.Context) {
		build.Context = filepath.Join(cwd, build.Context)
	}
	if name != "" {
		build.Tags = append(build.Tags, name)
	}
	return &build
}

type yamlBuildOptions BuildDefinition

// UnmarshalYAML implements [yaml.Unmarshaler] to parse build options from a string or a struct.
func (b *BuildDefinition) UnmarshalYAML(n *yaml.Node) (err error) {
	if n.Kind == yaml.ScalarNode {
		var s string
		err = n.Decode(&s)
		*b = BuildDefinition{Context: s}
		return err
	}
	var s yamlBuildOptions
	err = n.Decode(&s)
	if err != nil {
		return err
	}
	*b = BuildDefinition(s)
	return err
}

// ImageOptions stores options for creating/pulling an image.
type ImageOptions struct {
	Name         string
	Build        *BuildDefinition
	NoCache      bool
	ForceRebuild bool

	RegistryType     KubernetesRegistry
	RegistryURL      string
	RegistryInsecure bool
	BuildContainerID string
}

// ImageRemoveOptions stores options for removing an image.
type ImageRemoveOptions struct {
	Force            bool
	RegistryType     KubernetesRegistry
	RegistryURL      string
	BuildContainerID string
}

// ImageStatus defines image status on local machine.
type ImageStatus int64

// Image statuses.
const (
	ImageExists          ImageStatus = iota // ImageExists - image exists locally.
	ImageUnexpectedError                    // ImageUnexpectedError - image can't be pulled or retrieved.
	ImagePull                               // ImagePull - image is being pulled from the registry.
	ImageBuild                              // ImageBuild - image is being built.
	ImageRemoved                            // ImageRemoved - image was removed
	ImagePostpone                           // ImagePostpone - image action was postponed
)

// SystemInfo holds information about the container runner environment.
type SystemInfo struct {
	ID              string
	Name            string
	ServerVersion   string
	KernelVersion   string
	OperatingSystem string
	OSVersion       string
	OSType          string
	Architecture    string
	NCPU            int
	MemTotal        int64
	SecurityOptions []string
	Remote          bool // Remote defines if local or remote containers are spawned.
}

// ContainerListOptions stores options to request container list.
type ContainerListOptions struct {
	SearchName string
}

// ContainerListResult defines container list result.
type ContainerListResult struct {
	ID     string
	Names  []string
	Status string
}

// ImageStatusResponse stores the response when getting the image.
type ImageStatusResponse struct {
	Status   ImageStatus
	Progress *ImageProgressStream
}

// ImageProgressStream holds Image progress reader and a way to stream it to the given output.
type ImageProgressStream struct {
	io.ReadCloser
	streamer func(io.Reader, *launchr.Out) error
}

// Stream outputs progress to the given output.
func (p *ImageProgressStream) Stream(out *launchr.Out) error {
	if p.streamer == nil {
		_, err := io.Copy(out, p.ReadCloser)
		return err
	}
	return p.streamer(p.ReadCloser, out)
}

// Close closes the reader.
func (p *ImageProgressStream) Close() error {
	return p.ReadCloser.Close()
}

// ImageRemoveResponse stores response when removing the image.
type ImageRemoveResponse struct {
	Status ImageStatus
}

// ContainerPathStat is a type alias for container path stat result.
type ContainerPathStat struct {
	Name       string
	Size       int64
	Mode       os.FileMode
	Mtime      time.Time
	LinkTarget string
}

// CopyToContainerOptions is a type alias for container copy to container options.
type CopyToContainerOptions struct {
	AllowOverwriteDirWithFile bool
	CopyUIDGID                bool
}

// ContainerDefinition stores options for creating a new container.
type ContainerDefinition struct {
	Hostname      string
	ContainerName string
	Image         string
	ImageOptions  ImageOptions

	Entrypoint []string
	Command    []string
	WorkingDir string

	// @todo review binds and volumes, because binds won't work for remote environments.
	Binds   []string
	Volumes []ContainerVolume

	Streams ContainerStreamsOptions

	Env        []string
	User       string
	ExtraHosts []string
}

// ContainerVolume stores volume definition for a container.
type ContainerVolume struct {
	// Name is a volume name. Leave empty for an anonymous volume.
	Name string
	// MountPath is a path within the container at which the volume should be mounted. Must not contain ':'.
	MountPath string
}

// ContainerStreamsOptions stores options for attaching to streams of a running container.
type ContainerStreamsOptions struct {
	TTY    bool
	Stdin  bool
	Stdout bool
	Stderr bool
}

// ContainerStopOptions stores options to stop a container.
type ContainerStopOptions struct {
	Timeout *time.Duration
}
