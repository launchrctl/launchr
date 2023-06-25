package yamldiscovery

import (
	"context"
	"io/fs"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/launchrctl/launchr/core/action"
	"github.com/launchrctl/launchr/core/cli"
)

// DiscoveredActionsGroup is a cobra command group definition
var DiscoveredActionsGroup = &cobra.Group{
	ID:    "discovered_actions",
	Title: "Discovered actions:",
}

// CobraAddCommands implements launchr.CobraPlugin interface to provide discovered actions.
func (p *Plugin) CobraAddCommands(rootCmd *cobra.Command) error {
	// CLI command to discover actions in file structure and provide
	var discoverCmd = &cobra.Command{
		Use:   "discover",
		Short: "Discovers available actions in filesystem",
		RunE: func(cmd *cobra.Command, args []string) error {
			dp, err := GetDiscoveryPath()
			if err != nil {
				return err
			}
			cmds, err := discoverActions(os.DirFS(dp))
			if err != nil {
				return err
			}

			// @todo regenerate bin, show elapsed time.
			for _, a := range cmds {
				cli.Println("%s", a.CommandName)
			}

			return nil
		},
	}
	// Discover actions.
	rootCmd.AddCommand(discoverCmd)
	appFs := p.app.GetFS()
	if appFs == nil {
		dp, err := GetDiscoveryPath()
		if err != nil {
			return err
		}
		appFs = os.DirFS(dp)
	}
	cmds, err := discoverActions(appFs)
	if err != nil {
		return err
	}
	// Set cobra commands.
	rootCmd.AddGroup(DiscoveredActionsGroup)
	for _, cmdDef := range cmds {
		cobraCmd, err := getCobraActionImpl(p.app.GetCli(), cmdDef)
		if err != nil {
			return err
		}
		rootCmd.AddCommand(cobraCmd)
	}
	return nil
}

func discoverActions(fs fs.FS) ([]*action.Command, error) {
	return action.NewYamlDiscovery(fs).Discover()
}

// getCobraActionImpl returns cobra command implementation for an action.
func getCobraActionImpl(appCli cli.Cli, cmd *action.Command) (*cobra.Command, error) {
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
	ccmd := &cobra.Command{
		Use:  use,
		Args: cobra.ExactArgs(len(args)),
		// @todo: maybe we need a long template for arguments description
		Short:   getCobraCmdDesc(a.Title, a.Description),
		GroupID: DiscoveredActionsGroup.ID,
		RunE: func(ccmd *cobra.Command, args []string) error {
			// Don't show usage help on a runtime error.
			ccmd.SilenceUsage = true
			return runCmd(ccmd.Context(), appCli, cmd, args, options)
		},
	}

	for _, opt := range a.Options {
		options[opt.Name] = setCobraFlag(ccmd, opt)
	}

	return ccmd, nil
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

func getCobraCmdDesc(title string, desc string) string {
	parts := make([]string, 0, 2)
	if title != "" {
		parts = append(parts, title)
	}
	if desc != "" {
		parts = append(parts, desc)
	}
	return strings.Join(parts, ": ")
}
