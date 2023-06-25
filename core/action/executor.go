package action

import (
	"context"

	"github.com/launchrctl/launchr/core/cli"
)

// Executor is a common interface for all container executors.
type Executor interface {
	Exec(ctx context.Context, cli cli.Cli, cmd *Command) error
	Close() error
}
