package action

import (
	"context"

	"github.com/launchrctl/launchr/internal/launchr"
	"github.com/launchrctl/launchr/pkg/log"
	"github.com/launchrctl/launchr/pkg/types"
)

// RunEnvironment is a common interface for all action run environments.
type RunEnvironment interface {
	// Init prepares the run environment.
	Init() error
	// Execute runs action a in the environment and operates with IO through streams.
	Execute(ctx context.Context, a *Action) error
	// Close does wrap up operations.
	Close(ctx context.Context, a *Action) error
}

// RunEnvironmentFlags is an interface to define environment specific runtime configuration.
type RunEnvironmentFlags interface {
	RunEnvironment
	// FlagsDefinition provides definitions for action environment specific flags.
	FlagsDefinition() OptionsList
	// UseFlags sets environment configuration.
	UseFlags(flags TypeOpts) error
}

// ContainerRunEnvironment is an interface for container run environments.
type ContainerRunEnvironment interface {
	RunEnvironment
	// SetContainerNameProvider sets container name provider.
	SetContainerNameProvider(ContainerNameProvider)
	// AddImageBuildResolver adds an image build resolver to a chain.
	AddImageBuildResolver(ImageBuildResolver)
}

// ImageBuildResolver is an interface to resolve image build info from its source.
type ImageBuildResolver interface {
	// ImageBuildInfo takes image as name and provides build definition for that.
	ImageBuildInfo(image string) *types.BuildDefinition
}

// ChainImageBuildResolver is a image build resolver that takes first available image in the chain.
type ChainImageBuildResolver []ImageBuildResolver

// ImageBuildInfo implements ImageBuildResolver.
func (r ChainImageBuildResolver) ImageBuildInfo(image string) *types.BuildDefinition {
	for i := 0; i < len(r); i++ {
		if b := r[i].ImageBuildInfo(image); b != nil {
			return b
		}
	}
	return nil
}

// ConfigImagesKey is a field name in launchr config file.
const ConfigImagesKey = "images"

// ConfigImages is a container to parse launchr config in yaml format.
type ConfigImages map[string]*types.BuildDefinition

// LaunchrConfigImageBuildResolver is a resolver of image build in launchr config file.
type LaunchrConfigImageBuildResolver struct{ cfg launchr.Config }

// ImageBuildInfo implements ImageBuildResolver.
func (r LaunchrConfigImageBuildResolver) ImageBuildInfo(image string) *types.BuildDefinition {
	if r.cfg == nil {
		return nil
	}
	var images ConfigImages
	err := r.cfg.Get(ConfigImagesKey, &images)
	if err != nil {
		log.Warn("configuration file field %q is malformed", ConfigImagesKey)
		return nil
	}
	if b, ok := images[image]; ok {
		return b.ImageBuildInfo(image, r.cfg.DirPath())
	}
	for _, b := range images {
		for _, t := range b.Tags {
			if t == image {
				return b.ImageBuildInfo(image, r.cfg.DirPath())
			}
		}
	}
	return nil
}
