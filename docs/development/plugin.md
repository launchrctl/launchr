# Plugins

Plugins is a way to extend launchr functionality.  
The main difference from actions is that plugins must be written in `go` and run natively on a host machine.

## Available plugins

Available plugin types:

1. `OnAppInitPlugin` - runs on the application init stage.
2. `ActionDiscoveryPlugin` - discovers and provides actions
3. `CobraPlugin` - adds cobra cli functionality. Use `ActionDiscoveryPlugin` to add new actions.
4. `PersistentPreRunPlugin` - runs before cobra cobra command when all arguments are parsed.
5. `GeneratePlugin` = generates supporting files before build.

Plugin implementation examples:

1. [Default plugins](../plugins)
2. [Keyring](https://github.com/launchrctl/keyring)
3. [Compose](https://github.com/launchrctl/compose)
4. [Web](https://github.com/launchrctl/web)

## Plugin declaration

A plugin must implement `launchr.Plugin` interface. Here is an example:

```go
package example

import (
	"github.com/launchrctl/launchr"
)

func init() {
	launchr.RegisterPlugin(&Plugin{})
}

type Plugin struct{}

func (p *Plugin) PluginInfo() launchr.PluginInfo {
	return launchr.PluginInfo{}
}
```

## Action plugin

To implement an action using a plugin, define an action file. Here we create 2 actions:

<details>
<summary>action.login.yaml:</summary>

```yaml
runtime: plugin
action:
  title: "Keyring: Log in"
  description: >-
    Logs in to services like git, docker, etc.
  options:
    - name: url
      title: URL
      default: ""
    - name: username
      title: Username
      default: ""
    - name: password
      title: Password
      default: ""
```

</details>

<details>
<summary>action.logout.yaml:</summary>

```yaml
runtime: plugin
action:
  title: "Keyring: Log out"
  description: >-
    Logs out from a service
  arguments:
    - name: url
      title: URL
      description: URL to log out
      minLength: 1
  options:
    - name: all
      title: All
      description: Logs out from all services
      type: boolean
      default: false
```

</details>

The important thing here is - `runtime: plugin`.  
Next embed the actions into the code using `embed`. And define actions and their runtime in `DiscoverActions` implementing plugin `launchr.ActionDiscoveryPlugin`.

```go

import (
    _ "embed"
    
    "github.com/launchrctl/launchr"
)

var (
    //go:embed action.login.yaml
    actionLoginYaml []byte
    //go:embed action.logout.yaml
    actionLogoutYaml []byte
)

func init() {
    launchr.RegisterPlugin(&Plugin{})
}

// Plugin is [launchr.Plugin] plugin providing a keyring.
type Plugin struct {}

// DiscoverActions implements [launchr.ActionDiscoveryPlugin] interface.
func (p *Plugin) DiscoverActions(ctx context.Context) ([]*action.Action, error) {
	// Action login.
	loginCmd := action.NewFromYAML("keyring:login", actionLoginYaml)
	loginCmd.SetRuntime(action.NewFnRuntime(func(_ context.Context, a *action.Action) error {
		input := a.Input()
		creds := CredentialsItem{
			Username: input.Opt("username").(string),
			Password: input.Opt("password").(string),
			URL:      input.Opt("url").(string),
		}
		return nil
	}))

	// Action logout.
	logoutCmd := action.NewFromYAML("keyring:logout", actionLogoutYaml)
	logoutCmd.SetRuntime(action.NewFnRuntime(func(_ context.Context, a *action.Action) error {
		input := a.Input()
		all := input.Opt("all").(bool)
		if all == input.IsArgChanged("url") {
			return fmt.Errorf("please, either provide an URL or use --all flag")
		}
		url, _ := input.Arg("url").(string)
		return logout(p.k, url, all)
	}))

	return []*action.Action{
		loginCmd,
		logoutCmd,
	}, nil
}
```

## Value processors

Value processors are handlers applied to action parameters (arguments and options) to manipulate the data.

Define a processor using a generic `action.GenericValueProcessorOptions` or implement a fully custom. 
```go
type procTestReplaceOptions = *GenericValueProcessorOptions[struct {
    O string `yaml:"old" validate:"not-empty"`
    N string `yaml:"new"`
}]
```

Add the processor to the Action Manager:

```go
var am action.Manager
app.GetService(&am)

procReplace := GenericValueProcessor[procTestReplaceOptions]{
	Types: []jsonschema.Type{jsonschema.String},
	Fn: func(v any, opts procTestReplaceOptions, _ ValueProcessorContext) (any, error) {
		return strings.Replace(v.(string), opts.Fields.O, opts.Fields.N, -1), nil
	},
}

am.AddValueProcessor("test.replace", procReplace)
```

The generic `action.GenericValueProcessorOptions` gives a few benefits:
1. Property validation using `validate:"constraint1 constraint2"`. Available constraints:
   * `not-empty`
2. Parsing of the properties
3. Typed options when implementing the processor

See [build-in processors](../../plugins/builtinprocessors/plugin.go) for an example how to implement a value processor.
