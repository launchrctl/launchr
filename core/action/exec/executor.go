// Package exec has implementation of action execution in different environments.
package exec

import (
	"context"

	"github.com/launchrctl/launchr/core/action"
	"github.com/launchrctl/launchr/core/cli"
)

// Executor is a common interface for all container executors.
type Executor interface {
	Exec(ctx context.Context, cli cli.Cli, cmd *action.Command) error
	Close() error
}
