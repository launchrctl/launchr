# Plugin System Architecture

## Overview

Launchr implements a sophisticated plugin architecture that allows for extensible functionality through a weight-based plugin system with lifecycle hooks and interface-based design.

## Core Plugin Architecture

### Plugin Registration

Plugins register themselves globally during package initialization:

```go
// Global plugin registry
var registeredPlugins = make(PluginsMap)

func RegisterPlugin(p Plugin) {
    info := p.PluginInfo()
    InitPluginInfo(&info, p)
    if _, ok := registeredPlugins[info]; ok {
        panic(fmt.Errorf("plugin %q already registered", info))
    }
    registeredPlugins[info] = p
}
```

### Plugin Interface Hierarchy

#### Base Plugin Interface
```go
type Plugin interface {
    PluginInfo() PluginInfo
}

type PluginInfo struct {
    Name   string
    Weight int  // Used for ordering plugins
}
```

#### Specialized Plugin Interfaces

1. **OnAppInitPlugin** - Application initialization hooks
```go
type OnAppInitPlugin interface {
    Plugin
    OnAppInit(app App) error
}
```

2. **CobraPlugin** - Command-line interface integration
```go
type CobraPlugin interface {
    Plugin
    CobraAddCommands(rootCmd *Command) error
}
```

3. **PersistentPreRunPlugin** - Pre-execution hooks
```go
type PersistentPreRunPlugin interface {
    Plugin
    PersistentPreRun(cmd *Command, args []string) error
}
```

4. **DiscoveryPlugin** - Action discovery hooks
```go
type DiscoveryPlugin interface {
    Plugin
    DiscoverActions(ctx context.Context, manager Manager) ([]*Action, error)
}
```

5. **GeneratePlugin** - Code generation hooks
```go
type GeneratePlugin interface {
    Plugin
    Generate(config GenerateConfig) error
}
```

## Core Plugins

### 1. Action Naming Plugin (`plugins/actionnaming/`)
- **Purpose**: Configurable action ID transformation
- **Functionality**: Provides naming strategies for discovered actions

### 2. Actions Cobra Plugin (`plugins/actionscobra/`)
- **Purpose**: Cobra CLI integration for actions
- **Functionality**: Converts discovered actions into Cobra commands

### 3. YAML Discovery Plugin (`plugins/yamldiscovery/`)
- **Purpose**: YAML file discovery in filesystem
- **Functionality**: Discovers `action.yaml` files and loads action definitions

### 4. Built-in Processors Plugin (`plugins/builtinprocessors/`)
- **Purpose**: Value processing for action parameters
- **Functionality**: Provides processors for different parameter types

### 5. Builder Plugin (`plugins/builder/`)
- **Purpose**: Code generation and template functionality
- **Functionality**: Builds custom launchr binaries with embedded plugins

### 6. Verbosity Plugin (`plugins/verbosity/`)
- **Purpose**: Logging level management
- **Functionality**: Handles verbosity flags and log level configuration

## Plugin Lifecycle

### 1. Registration Phase
```go
func init() {
    launchr.RegisterPlugin(&myPlugin{})
}
```

### 2. Discovery Phase
```go
func (app *appImpl) init() error {
    // Get plugins by type
    plugins := launchr.GetPluginByType[OnAppInitPlugin](app.pluginMngr)
    
    // Execute in weight order
    for _, p := range plugins {
        if err = p.V.OnAppInit(app); err != nil {
            return err
        }
    }
}
```

### 3. Execution Phase
```go
func (app *appImpl) exec() error {
    // Add commands from plugins
    plugins := launchr.GetPluginByType[CobraPlugin](app.pluginMngr)
    for _, p := range plugins {
        if err := p.V.CobraAddCommands(app.cmd); err != nil {
            return err
        }
    }
}
```

## Plugin Weight System

Plugins are ordered by weight for execution:
- **Lower weight** = **Higher priority** (executed first)
- **Default weight**: Usually 0 or positive integers
- **System plugins**: Often use negative weights for high priority

## Plugin Implementation Example

```go
type MyPlugin struct {
    name string
}

func (p *MyPlugin) PluginInfo() launchr.PluginInfo {
    return launchr.PluginInfo{
        Name:   p.name,
        Weight: 100,
    }
}

func (p *MyPlugin) OnAppInit(app launchr.App) error {
    // Plugin initialization logic
    return nil
}

func init() {
    launchr.RegisterPlugin(&MyPlugin{name: "my-plugin"})
}
```

## Plugin Service Integration

Plugins can access application services through dependency injection:

```go
func (p *MyPlugin) OnAppInit(app launchr.App) error {
    var config launchr.Config
    app.GetService(&config)
    
    var manager action.Manager
    app.GetService(&manager)
    
    // Use services...
    return nil
}
```

## Best Practices

### Plugin Design
1. **Single Responsibility**: Each plugin should have one clear purpose
2. **Minimal Dependencies**: Avoid tight coupling between plugins
3. **Error Handling**: Always return meaningful errors
4. **Weight Selection**: Choose weights thoughtfully for proper ordering

### Plugin Registration
```go
// Good: Clear registration in init()
func init() {
    launchr.RegisterPlugin(&wellNamedPlugin{})
}

// Bad: Registration outside init()
func RegisterMyPlugin() {
    launchr.RegisterPlugin(&myPlugin{})
}
```

### Plugin Interface Implementation
```go
// Good: Implement only needed interfaces
type MyDiscoveryPlugin struct{}

func (p *MyDiscoveryPlugin) PluginInfo() launchr.PluginInfo { ... }
func (p *MyDiscoveryPlugin) DiscoverActions(...) ([]*Action, error) { ... }

// Bad: Implementing unnecessary interfaces
type MyPlugin struct{}
func (p *MyPlugin) OnAppInit(...) error { return nil } // Empty implementation
```

## Advanced Features

### Plugin Composition
Plugins can implement multiple interfaces:

```go
type MultiInterfacePlugin struct{}

func (p *MultiInterfacePlugin) PluginInfo() launchr.PluginInfo { ... }
func (p *MultiInterfacePlugin) OnAppInit(app launchr.App) error { ... }
func (p *MultiInterfacePlugin) CobraAddCommands(cmd *Command) error { ... }
```

### Plugin Dependencies
While not explicitly supported, plugins can coordinate through:
- **Weight ordering**: Lower weight plugins run first
- **Service sharing**: Common services accessed through app
- **Configuration**: Shared configuration through Config service

## Limitations and Considerations

### Current Limitations
1. **Weight-based ordering**: Primitive dependency resolution
2. **Global registration**: All plugins registered globally
3. **No unloading**: Plugins cannot be dynamically unloaded
4. **Panic on conflicts**: Duplicate plugin names cause panics

### Improvement Opportunities
1. **Dependency graphs**: Explicit plugin dependencies
2. **Plugin discovery**: Dynamic plugin loading from files
3. **Plugin isolation**: Namespace isolation between plugins
4. **Hot reloading**: Dynamic plugin reloading support

## Plugin Development Workflow

1. **Define Purpose**: Clear single responsibility
2. **Choose Interfaces**: Implement only necessary plugin interfaces
3. **Select Weight**: Choose appropriate execution order
4. **Implement Logic**: Core plugin functionality
5. **Register**: Add registration in `init()`
6. **Test**: Ensure plugin works in isolation and with others
7. **Document**: Provide clear documentation and examples