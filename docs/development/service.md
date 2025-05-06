# Services

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
    "github.com/launchrctl/launchr"
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
    "github.com/launchrctl/launchr"
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
