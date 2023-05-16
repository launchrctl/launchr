# Launchr

## Usage

There are the following build options:
1. `-o, --output OUTPUT` - result file
2. `-p, --plugin PLUGIN[@v1.1]` - use plugin in the built launchr. The flag may be specified multiple times.
    ```shell
    launchr \
      -p github.com/launchrctl/launchr \ 
      -p github.com/launchrctl/launchr@v1
    ```
3. `-r, --replace OLD=NEW` - replace go dependency, see [go mod edit -replace](https://go.dev/ref/mod#go-mod-edit). The flag may be specified multiple times.

    The directive may be used to replace a private repository with a local path or to set a specific version of a module. Example:
    ```shell
    launchr --replace github.com/launchrctl/launchr=/path/to/local/dir
    launchr --replace github.com/launchrctl/launchr=github.com/launchrctl/launchr@v1.2
    ```

4. `-d, --debug` - include debug flags into the build to support go debugging like [Delve](https://github.com/go-delve/delve).
    Without the flag, all debugging info is trimmed.
5. `-v, --version` - output Launchr version
6. `-h, --help` - output help message

## Plugins

Plugins is a way to extend launchr functionality.  
The main difference from actions is that plugins must be written in `go` and run natively on a host machine.

The plugin must implement `core.Plugin` interface. Here is an example:
```go
package example

import (
    ...
    launchr "github.com/launchrctl/launchr/core"
    ...
)

func init() {
	launchr.RegisterPlugin(&Plugin{})
}

// Plugin is a plugin to discover actions defined in yaml.
type Plugin struct {
	app *launchr.App
}

// PluginInfo implements core.Plugin interface.
func (p *Plugin) PluginInfo() launchr.PluginInfo {
	return launchr.PluginInfo{
		ID: ID,
	}
}

// InitApp implements core.Plugin interface to provide discovered actions.
func (p *Plugin) InitApp(app *launchr.App) error {
	p.app = app
	return nil
}
```

See `yamldiscovery` plugin for an example.
