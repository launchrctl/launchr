# Launchr

## Build plugin

There are the following build options:
1. `-o, --output OUTPUT` - result file. If empty, application name is used.
2. `-n, --name NAME` - application name.
3. `-p, --plugin PLUGIN[@v1.1]` - use plugin in the built launchr. The flag may be specified multiple times.
    ```shell
    launchr build \
      -p github.com/launchrctl/launchr \ 
      -p github.com/launchrctl/launchr@v0.1.0
    ```
4. `-r, --replace OLD=NEW` - replace go dependency, see [go mod edit -replace](https://go.dev/ref/mod#go-mod-edit). The flag may be specified multiple times.

    The directive may be used to replace a private repository with a local path or to set a specific version of a module. Example:
    ```shell
    launchr build --replace github.com/launchrctl/launchr=/path/to/local/dir
    launchr build --replace github.com/launchrctl/launchr=github.com/launchrctl/launchr@v0.2.0
    ```

5. `-d, --debug` - include debug flags into the build to support go debugging like [Delve](https://github.com/go-delve/delve).
    Without the flag, all debugging info is trimmed.
6. `-h, --help` - output help message

## Plugins

Plugins is a way to extend launchr functionality.  
The main difference from actions is that plugins must be written in `go` and run natively on a host machine.

A plugin must implement `launchr.Plugin` interface. Here is an example:
```go
package example

import (
    ...
    "github.com/launchrctl/launchr"
    ...
)

func init() {
	launchr.RegisterPlugin(&Plugin{})
}

type Plugin struct {}

func (p *Plugin) PluginInfo() launchr.PluginInfo {
	return launchr.PluginInfo{}
}
```

Available plugin types:
1. `OnAppInitPlugin`
2. `CobraPlugin`
3. `GeneratePlugin`

Plugin implementation examples:
1. [yamldiscovery](../plugins/yamldiscovery)
2. [Keyring](https://github.com/launchrctl/keyring)
2. [Compose](https://github.com/launchrctl/compose)

## Services

Service is a launchr interface to share functionality between plugins. It is a simple Dependency Injection mechanism.

In launchr there are several core services:
1. `launchr.Config` - stores global launchr configuration, see [config documentation](config.global.md).
2. `action.Manager` - manages available actions, see [actions documentation](actions.md)
3. `launchr.PluginManager` - stores all registered plugins.

Services available with plugins:
1. [Keyring](https://github.com/launchrctl/keyring) - provides functionality to store passwords.

### How to use services

Service can be retrieved from the `launchr.App`. It is important to use a unique interface to retrieve the specific service 
from the app.

```go
package example

import (
    ...
    "github.com/launchrctl/launchr"
    ...
)

// Get a service from the App.
func (p *Plugin) OnAppInit(app launchr.App) error {
	var cfg launchr.Config
	app.GetService(&cfg) // Pass a pointer to init the value.
	return nil
}
```

### How to implement a service
A service must implement `launchr.Service` interface. Here is an example:

```go
package example

import (
    ...
    "github.com/launchrctl/launchr"
    ...
)

// Define a service and implement service interface.
// It is important to have a unique interface, the service is identified by it in launchr.GetService().
type ExampleService interface {
	launchr.Service // Inherit launchr.Service
	// Provide other methods if needed.
}

type exampleSrvImpl struct {
    // ...
}

func (ex *exampleSrvImpl) ServiceInfo() launchr.ServiceInfo {
	return launchr.ServiceInfo{}
}

// Register a service inside launchr.
func (p *Plugin) OnAppInit(app launchr.App) error {
	srv := &exampleSrvImpl{}
	app.AddService(srv)
	return nil
}
```