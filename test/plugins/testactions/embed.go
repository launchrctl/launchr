package testactions

import (
	"embed"

	"github.com/launchrctl/launchr/internal/launchr"
	"github.com/launchrctl/launchr/pkg/action"
)

//go:embed test-registered-embed-fs/*
var registeredEmbedFS embed.FS

//go:embed embed-fs/*
var embedActionsFS embed.FS

func embedContainerAction() *action.Action {
	// Create an action from FS.
	// Use subdirectory so the content is available in the root "./".
	subfs := launchr.MustSubFS(embedActionsFS, "embed-fs/action-container")
	a, err := action.NewYAMLFromFS("test-embed-fs:container", subfs)
	if err != nil {
		panic(err)
	}
	return a
}
