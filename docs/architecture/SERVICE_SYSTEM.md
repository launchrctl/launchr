# Service System Architecture

## Overview

Launchr implements a service-oriented architecture using dependency injection pattern. The system provides a type-safe way to register and retrieve services throughout the application.

## Core Service Architecture

### Service Interface
```go
type Service interface {
    ServiceInfo() ServiceInfo
}

type ServiceInfo struct {
    Name        string
    Description string
}
```

### Service Container
The application acts as a service container:

```go
type appImpl struct {
    services   map[ServiceInfo]Service
    // ... other fields
}

func (app *appImpl) AddService(s Service) {
    info := s.ServiceInfo()
    launchr.InitServiceInfo(&info, s)
    if _, ok := app.services[info]; ok {
        panic(fmt.Errorf("service %s already exists", info))
    }
    app.services[info] = s
}

func (app *appImpl) GetService(v any) {
    // Reflection-based service retrieval
    for _, srv := range app.services {
        st := reflect.TypeOf(srv)
        if st.AssignableTo(stype) {
            reflect.ValueOf(v).Elem().Set(reflect.ValueOf(srv))
            return
        }
    }
    panic(fmt.Sprintf("service %q does not exist", stype))
}
```

## Core Services

### 1. Configuration Service
**Interface**: `Config`
**Implementation**: `launchr.ConfigFromFS`
**Purpose**: YAML-based configuration management

```go
type Config interface {
    Exists(key string) bool
    Get(key string, v any) error
    Path(parts ...string) string
}
```

**Usage**:
```go
var config launchr.Config
app.GetService(&config)

var timeout time.Duration
config.Get("action.timeout", &timeout)
```

### 2. Action Manager Service
**Interface**: `Manager`
**Implementation**: `action.NewManager`
**Purpose**: Action lifecycle management

```go
type Manager interface {
    All() map[string]*Action
    Get(id string) (*Action, bool)
    Add(*Action) error
    Delete(id string)
    Discover(ctx context.Context) ([]*Action, error)
    // ... other methods
}
```

**Usage**:
```go
var manager action.Manager
app.GetService(&manager)

actions, err := manager.Discover(ctx)
```

### 3. Plugin Manager Service
**Interface**: `PluginManager`
**Implementation**: `launchr.NewPluginManagerWithRegistered`
**Purpose**: Plugin management and discovery

```go
type PluginManager interface {
    GetPlugins() PluginsMap
    GetPluginByType(pluginType reflect.Type) []PluginMapItem
}
```

**Usage**:
```go
var pluginMgr launchr.PluginManager
app.GetService(&pluginMgr)

plugins := launchr.GetPluginByType[OnAppInitPlugin](pluginMgr)
```

## Service Registration

Services are registered during application initialization:

```go
func (app *appImpl) init() error {
    // Create services
    config := launchr.ConfigFromFS(os.DirFS(app.cfgDir))
    actionMngr := action.NewManager(
        action.WithDefaultRuntime(config),
        action.WithContainerRuntimeConfig(config, name+"_"),
    )

    // Register services
    app.AddService(actionMngr)
    app.AddService(app.pluginMngr)
    app.AddService(config)

    return nil
}
```

## Service Usage Patterns

### In Plugins
```go
func (p *MyPlugin) OnAppInit(app launchr.App) error {
    // Get required services
    var config launchr.Config
    app.GetService(&config)
    
    var manager action.Manager
    app.GetService(&manager)
    
    // Use services
    return p.initializeWithServices(config, manager)
}
```

### In Application Code
```go
func (app *appImpl) someMethod() error {
    var manager action.Manager
    app.GetService(&manager)
    
    return manager.SomeOperation()
}
```

## Service Lifecycle

### 1. Creation Phase
Services are created during app initialization:
```go
config := launchr.ConfigFromFS(os.DirFS(app.cfgDir))
actionMngr := action.NewManager(options...)
```

### 2. Registration Phase
Services are registered with the container:
```go
app.AddService(config)
app.AddService(actionMngr)
```

### 3. Usage Phase
Services are retrieved when needed:
```go
var config launchr.Config
app.GetService(&config)
```

## Service Implementation Example

```go
type MyService struct {
    name string
    config Config
}

func (s *MyService) ServiceInfo() launchr.ServiceInfo {
    return launchr.ServiceInfo{
        Name:        "my-service",
        Description: "Example service implementation",
    }
}

func (s *MyService) DoSomething() error {
    // Service logic
    return nil
}

func NewMyService(config launchr.Config) *MyService {
    return &MyService{
        name:   "my-service",
        config: config,
    }
}
```

## Service Dependencies

Services can depend on other services through constructor injection:

```go
func NewActionManager(config Config, pluginMgr PluginManager) Manager {
    return &actionManager{
        config:    config,
        pluginMgr: pluginMgr,
    }
}
```

## Best Practices

### Service Design
1. **Single Responsibility**: Each service should have one clear purpose
2. **Interface Segregation**: Define focused interfaces
3. **Dependency Injection**: Use constructor injection for dependencies
4. **Immutable State**: Prefer immutable service configuration

### Service Registration
```go
// Good: Register services in logical order
app.AddService(config)      // Base service
app.AddService(pluginMgr)   // Core service
app.AddService(actionMgr)   // Dependent service

// Bad: Random registration order
app.AddService(actionMgr)   // Depends on config
app.AddService(config)      // Should be registered first
```

### Service Retrieval
```go
// Good: Check for service existence
var config launchr.Config
app.GetService(&config)
if config == nil {
    return errors.New("config service not available")
}

// Bad: Assume service exists (will panic if missing)
var config launchr.Config
app.GetService(&config)
config.Get("key", &value) // Potential panic
```

## Strengths

1. **Type Safety**: Reflection-based but type-safe service retrieval
2. **Dependency Injection**: Clean separation of concerns
3. **Service Discovery**: Easy access to registered services
4. **Interface-Based**: Services defined by contracts, not implementations

## Limitations

1. **Reflection Overhead**: Runtime reflection for service retrieval
2. **Panic on Missing**: Missing services cause panics
3. **No Lifecycle Management**: Services don't have explicit lifecycle hooks
4. **Single Instance**: Each service type can only have one instance

## Improvement Opportunities

### 1. Service Lifecycle Management
```go
type ServiceLifecycle interface {
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    Health() error
}
```

### 2. Service Scoping
```go
type ServiceScope string

const (
    ScopeSingleton ServiceScope = "singleton"
    ScopeTransient ServiceScope = "transient"
    ScopeScoped    ServiceScope = "scoped"
)
```

### 3. Service Factories
```go
type ServiceFactory interface {
    Create(container ServiceContainer) (Service, error)
    Scope() ServiceScope
}
```

### 4. Graceful Error Handling
```go
func (app *appImpl) GetService(v any) error {
    // Return error instead of panic
    if service, exists := app.findService(v); exists {
        setValue(v, service)
        return nil
    }
    return fmt.Errorf("service %T not found", v)
}
```

## Advanced Patterns

### Service Composition
```go
type CompositeService struct {
    config  Config
    manager Manager
    logger  Logger
}

func (s *CompositeService) ServiceInfo() launchr.ServiceInfo {
    return launchr.ServiceInfo{
        Name: "composite-service",
    }
}
```

### Service Delegation
```go
type DelegatingService struct {
    delegate Service
}

func (s *DelegatingService) DoWork() error {
    // Add cross-cutting concerns
    log.Debug("starting work")
    defer log.Debug("finished work")
    
    return s.delegate.DoWork()
}
```

The service system provides a clean, type-safe way to manage dependencies and share functionality across the application while maintaining loose coupling between components.