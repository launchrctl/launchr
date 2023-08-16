package action

import (
	"context"
	"reflect"
	"strings"

	"github.com/spf13/cobra"

	"github.com/launchrctl/launchr/internal/launchr"
	"github.com/launchrctl/launchr/pkg/cli"
	"github.com/launchrctl/launchr/pkg/jsonschema"
	"github.com/launchrctl/launchr/pkg/log"
)

// CobraImpl returns cobra command implementation for an action command.
func CobraImpl(cmd *Command, appCli cli.Streams, cfg launchr.Config, group *cobra.Group) (*cobra.Command, error) {
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
			return runCmd(ccmd.Context(), cmd, appCli, cfg, args, options)
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

func runCmd(ctx context.Context, cmd *Command, appCli cli.Streams, cfg launchr.Config, args []string, opts map[string]interface{}) error {
	// Save and validate input.
	cmd.SetArgsInput(args)
	cmd.SetOptsInput(derefOpts(opts))
	if err := cmd.Compile(); err != nil {
		return err
	}
	if err := cmd.ValidateInput(); err != nil {
		return err
	}

	r, err := NewDockerExecutor()
	if err != nil {
		return err
	}
	defer r.Close()
	if r, ok := r.(launchr.ConfigAware); ok {
		r.SetLaunchrConfig(cfg)
	}

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

func setFlag(ccmd *cobra.Command, opt *Option) interface{} {
	var val interface{}
	desc := getDesc(opt.Title, opt.Description)
	switch opt.Type {
	case jsonschema.String:
		val = ccmd.Flags().String(opt.Name, opt.Default.(string), desc)
	case jsonschema.Integer:
		val = ccmd.Flags().Int(opt.Name, opt.Default.(int), desc)
	case jsonschema.Number:
		val = ccmd.Flags().Float64(opt.Name, opt.Default.(float64), desc)
	case jsonschema.Boolean:
		val = ccmd.Flags().Bool(opt.Name, opt.Default.(bool), desc)
	case jsonschema.Array:
		// @todo parse results to requested type somehow
		val = ccmd.Flags().StringSlice(opt.Name, opt.Default.([]string), desc)
	default:
		log.Panic("json schema type %s is not implemented", opt.Type)
	}
	if opt.Required {
		_ = ccmd.MarkFlagRequired(opt.Name)
	}
	return val
}

func derefOpts(opts map[string]interface{}) map[string]interface{} {
	der := make(map[string]interface{})
	for k, v := range opts {
		der[k] = derefOpt(v)
	}
	return der
}

func derefOpt(v interface{}) interface{} {
	switch v := v.(type) {
	case *string:
		return *v
	case *bool:
		return *v
	case *int:
		return *v
	case *float64:
		return *v
	case *[]string:
		return *v
	default:
		if reflect.ValueOf(v).Kind() == reflect.Ptr {
			log.Panic("error on a value dereferencing: unsupported %T", v)
		}
		return v
	}
}
