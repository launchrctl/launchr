// Package action_embedfs contains an example how to create
// a plugin that provides actions in embed fs.
package action_embedfs //nolint:revive // using underscore for better example naming

import (
	"context"
	"embed"

	"github.com/launchrctl/launchr"
	"github.com/launchrctl/launchr/pkg/action"
)

// Embed the `actions` directory.
// The directory can have any name or contain multiple subdirectories with action definitions.
// In this example, we include many and specifically reference the `actions/example1` subdirectory
// using `launchr.MustSubFS(actionfs, "actions/example1")`. The action ID is set explicitly to `example:embedfs_1`.
//
// It is recommended to use a subdirectory (like `action`) instead of the project root
// to limit the scope of embedded files.
//
//go:embed actions
var actionfs embed.FS

func init() {
	launchr.RegisterPlugin(&Plugin{})
}

// Plugin is [launchr.Plugin] providing action.
type Plugin struct{}

// PluginInfo implements [launchr.Plugin] interface.
func (p *Plugin) PluginInfo() launchr.PluginInfo {
	return launchr.PluginInfo{}
}

// DiscoverActions implements [launchr.ActionDiscoveryPlugin] interface.
func (p *Plugin) DiscoverActions(_ context.Context) ([]*action.Action, error) {
	// Use subdirectory so the content is available in the root "./".
	a1, err := action.NewYAMLFromFS("example:embedfs_1", launchr.MustSubFS(actionfs, "actions/example1"))
	if err != nil {
		return nil, err
	}
	a2, err := action.NewYAMLFromFS("example:embedfs_2", launchr.MustSubFS(actionfs, "actions/example2"))
	if err != nil {
		return nil, err
	}
	return []*action.Action{a1, a2}, nil
}
