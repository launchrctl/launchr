// Package action_result_schema provides an example of creating an action
// with structured output using result schema.
// It demonstrates how to define a result schema in action.yaml and return
// structured data that can be output as JSON with --json flag.
package action_result_schema //nolint:revive // using underscore for better example naming

import (
	"context"
	_ "embed"
	"fmt"
	"time"

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

// Plugin is [launchr.Plugin] providing example plugin action with result schema.
type Plugin struct{}

// PluginInfo implements [launchr.Plugin] interface.
func (p *Plugin) PluginInfo() launchr.PluginInfo {
	return launchr.PluginInfo{}
}

// Item represents a generated item.
type Item struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

// Summary contains summary statistics.
type Summary struct {
	Total  int    `json:"total"`
	Prefix string `json:"prefix"`
}

// Result is the structured output of the action.
type Result struct {
	Items   []Item  `json:"items"`
	Summary Summary `json:"summary"`
}

// DiscoverActions implements [launchr.ActionDiscoveryPlugin] interface.
func (p *Plugin) DiscoverActions(_ context.Context) ([]*action.Action, error) {
	// Create the action from yaml definition.
	a := action.NewFromYAML("example:result-schema", actionYaml)

	// Use NewFnRuntimeWithResult to create a runtime that returns structured output.
	a.SetRuntime(action.NewFnRuntimeWithResult(func(_ context.Context, a *action.Action) (any, error) {
		input := a.Input()

		// Get input parameters.
		count := input.Arg("count").(int)
		prefix := input.Opt("prefix").(string)

		// Generate items.
		items := make([]Item, count)
		now := time.Now()
		for i := 0; i < count; i++ {
			items[i] = Item{
				ID:        i + 1,
				Name:      fmt.Sprintf("%s_%d", prefix, i+1),
				CreatedAt: now.Add(time.Duration(i) * time.Second).Format(time.RFC3339),
			}
		}

		// Build result.
		result := Result{
			Items: items,
			Summary: Summary{
				Total:  count,
				Prefix: prefix,
			},
		}

		// Print human-readable output for non-JSON mode.
		launchr.Term().Printfln("Generated %d items with prefix %q:", count, prefix)
		for _, item := range items {
			launchr.Term().Printfln("  - %s (id=%d)", item.Name, item.ID)
		}

		// Return the structured result.
		// This will be captured by the runtime and serialized as JSON when --json is used.
		return result, nil
	}))

	return []*action.Action{a}, nil
}
