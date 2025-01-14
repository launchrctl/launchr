// Package actionscobra is a launchr plugin providing cobra interface to actions.
package actionscobra

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/launchrctl/launchr/internal/launchr"
	"github.com/launchrctl/launchr/pkg/action"
)

var (
	errDiscoveryTimeout = "action discovery timeout exceeded"
)

// ActionsGroup is a command group definition.
var ActionsGroup = &launchr.CommandGroup{
	ID:    "actions",
	Title: "Actions:",
}

func init() {
	launchr.RegisterPlugin(&Plugin{})
}

// Plugin is [launchr.Plugin] to add command line interface to actions.
type Plugin struct {
	app launchr.AppInternal
	am  action.Manager
	pm  launchr.PluginManager
}

// PluginInfo implements [launchr.Plugin] interface.
func (p *Plugin) PluginInfo() launchr.PluginInfo {
	return launchr.PluginInfo{
		// Set to max to run discovery after all.
		Weight: math.MaxInt,
	}
}

// OnAppInit implements [launchr.Plugin] interface.
func (p *Plugin) OnAppInit(app launchr.App) error {
	p.app = app.(launchr.AppInternal)
	app.GetService(&p.am)
	app.GetService(&p.pm)
	return p.discoverActions()
}

func (p *Plugin) discoverActions() (err error) {
	app := p.app
	early := app.CmdEarlyParsed()
	// Skip actions discovering.
	if early.IsVersion || early.IsGen {
		return err
	}
	var discovered []*action.Action
	// @todo configure timeout from flags
	// Define timeout for cases when we may traverse the whole FS, e.g. in / or home.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for _, p := range launchr.GetPluginByType[action.DiscoveryPlugin](p.pm) {
		actions, errDis := p.V.DiscoverActions(ctx)
		if errDis != nil {
			return errDis
		}
		discovered = append(discovered, actions...)
	}
	// Failed to discover actions in reasonable time.
	if errCtx := ctx.Err(); errCtx != nil {
		return errors.New(errDiscoveryTimeout)
	}

	// Add discovered actions.
	for _, a := range discovered {
		err = p.am.Add(a)
		if err != nil {
			launchr.Log().Warn("action was skipped due to error", "action_id", a.ID, "error", err)
			launchr.Term().Warning().Printfln("Action %q was skipped:\n%v", a.ID, err)
			continue
		}
	}

	// Alter all registered actions.
	for _, p := range launchr.GetPluginByType[action.AlterActionsPlugin](p.pm) {
		err = p.V.AlterActions()
		if err != nil {
			return err
		}
	}
	// @todo maybe cache discovery result for performance.
	return err
}

// CobraAddCommands implements [launchr.CobraPlugin] interface to add actions in command line.
func (p *Plugin) CobraAddCommands(rootCmd *launchr.Command) error {
	app := p.app
	early := app.CmdEarlyParsed()
	// Convert actions to cobra commands.
	// Check the requested command to see what actions we must actually load.
	var actions map[string]*action.Action
	if early.Command != "" {
		// Check if an alias was provided to find the real action.
		early.Command = p.am.GetIDFromAlias(early.Command)
		a, ok := p.am.Get(early.Command)
		if ok {
			// Use only the requested action.
			actions = map[string]*action.Action{a.ID: a}
		} else {
			// Action was not requested, no need to load them.
			return nil
		}
	} else {
		// Load all.
		actions = p.am.All()
	}

	// @todo consider cobra completion and caching between runs.
	if len(actions) > 0 {
		rootCmd.AddGroup(ActionsGroup)
	}
	streams := p.app.Streams()
	for _, a := range actions {
		cmd, err := CobraImpl(a, streams)
		if err != nil {
			launchr.Log().Warn("action was skipped due to error", "action_id", a.ID, "error", err)
			launchr.Term().Warning().Printfln("Action %q was skipped:\n%v", a.ID, err)
			continue
		}
		cmd.GroupID = ActionsGroup.ID
		rootCmd.AddCommand(cmd)
	}
	return nil
}
