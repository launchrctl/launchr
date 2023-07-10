// Package cobraadapter provides an adapter from launchr actions to cobra commands.
package cobraadapter

import (
	"context"
	"strings"

	"github.com/spf13/cobra"

	"github.com/launchrctl/launchr/pkg/action"
	"github.com/launchrctl/launchr/pkg/cli"
)

// GetActionImpl returns cobra command implementation for an action.
func GetActionImpl(appCli cli.Cli, cmd *action.Command, group *cobra.Group) (*cobra.Command, error) {
	if err := cmd.Compile(); err != nil {
		return nil, err
	}
	a := cmd.Action()
	args := a.Arguments
	use := cmd.CommandName
	for _, p := range args {
		use += " " + p.Name
	}
	options := make(map[string]interface{})
	cobraCmd := &cobra.Command{
		Use:  use,
		Args: cobra.ExactArgs(len(args)),
		// @todo: maybe we need a long template for arguments description
		Short: getDesc(a.Title, a.Description),
		RunE: func(ccmd *cobra.Command, args []string) error {
			// Don't show usage help on a runtime error.
			ccmd.SilenceUsage = true
			return runCmd(ccmd.Context(), appCli, cmd, args, options)
		},
	}
	if group != nil {
		cobraCmd.GroupID = group.ID
	}

	for _, opt := range a.Options {
		options[opt.Name] = setFlag(cobraCmd, opt)
	}

	return cobraCmd, nil
}

func runCmd(ctx context.Context, appCli cli.Cli, cmd *action.Command, args []string, opts map[string]interface{}) error {
	// Save and validate input.
	cmd.SetArgsInput(args)
	cmd.SetOptsInput(derefOpts(opts))
	if err := cmd.Compile(); err != nil {
		return err
	}
	if err := cmd.ValidateInput(); err != nil {
		return err
	}

	r, err := action.NewDockerExecutor()
	if err != nil {
		return err
	}
	defer r.Close()

	// Run the command.
	return r.Exec(ctx, appCli, cmd)
}

func getDesc(title string, desc string) string {
	parts := make([]string, 0, 2)
	if title != "" {
		parts = append(parts, title)
	}
	if desc != "" {
		parts = append(parts, desc)
	}
	return strings.Join(parts, ": ")
}
