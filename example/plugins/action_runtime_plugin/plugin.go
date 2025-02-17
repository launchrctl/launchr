// Package action_runtime_plugin provides an example of creating an action
// with the runtime type "plugin".
// It includes a basic implementation and usage of input parameters.
package action_runtime_plugin //nolint:revive // using underscore for better example naming

import (
	"context"
	_ "embed"

	"github.com/launchrctl/launchr"
	"github.com/launchrctl/launchr/pkg/action"
)

// Embed action yaml file. It is later used in DiscoverActions.
//
//go:embed action.yaml
var actionYaml []byte

func init() {
	launchr.RegisterPlugin(&Plugin{})
}

// Plugin is [launchr.Plugin] providing example plugin action.
type Plugin struct{}

// PluginInfo implements [launchr.Plugin] interface.
func (p *Plugin) PluginInfo() launchr.PluginInfo {
	return launchr.PluginInfo{}
}

type exampleArgs struct {
	arg     string
	argOpt  string
	optInt  int
	optBool bool
}

// DiscoverActions implements [launchr.ActionDiscoveryPlugin] interface.
func (p *Plugin) DiscoverActions(_ context.Context) ([]*action.Action, error) {
	// Create the action from yaml definition.
	a := action.NewFromYAML("example:runtime-plugin", actionYaml)

	// Define the callback function for the runtime to execute the code.
	a.SetRuntime(action.NewFnRuntime(func(_ context.Context, a *action.Action) error {
		// Ensure the action `a` comes from the function argument.
		// Avoid shadowing the `a` variable to preserve the correct input.
		input := a.Input()

		// Since the action arguments and options are defined in the YAML file,
		// we assert their presence in the input.
		// If a type assertion panic occurs, double-check that the names match here and in the YAML file.
		args := exampleArgs{
			arg:     input.Arg("arg").(string),
			argOpt:  input.Arg("arg_optional").(string),
			optInt:  input.Opt("opt_int").(int),
			optBool: input.Opt("opt_bool").(bool),
		}

		// Check if the optional argument was explicitly set by the user.
		if input.IsArgChanged("arg_optional") {
			launchr.Term().Warning().Printfln("arg_optional is overridden to %s", args.argOpt)
		}

		// Verify that the option is set, as it has no default value and is nil by default.
		// A nil value cannot be directly type-asserted to a string.
		optNoDef := input.Opt("opt_no_default")
		if optNoDef != nil {
			launchr.Term().Warning().Printfln("opt_no_default is set to %s", optNoDef.(string))
		}

		return example(args)
	}))
	return []*action.Action{a}, nil
}

func example(args exampleArgs) error {
	// Code to execute by command goes here.
	launchr.Term().Println()
	launchr.Term().Printfln("The input: %v", args)

	return nil
}
