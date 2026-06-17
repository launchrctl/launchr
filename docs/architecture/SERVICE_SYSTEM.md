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
    pkgPath  string  // Set automatically via InitServiceInfo()
    typeName string  // Set automatically via InitServiceInfo()
}
```

### ServiceCreate Interface
Services can implement `ServiceCreate` for lazy initialization:
```go
type ServiceCreate interface {
    Service
    ServiceCreate(svc *ServiceManager) Service
}
```

### Service Container
The `ServiceManager` is a basic Dependency Injection container:

```go
type ServiceManager struct {
    services map[ServiceInfo]Service
}

func (sm *ServiceManager) Add(s Service) {
    info := sm.serviceInfo(s)
    if _, ok := sm.services[info]; ok {
        panic(fmt.Errorf("service %s already exists, review your code", info))
    }
    sm.services[info] = s
}

func (sm *ServiceManager) Get(v any) {
    // 1. Try direct lookup by ServiceInfo
    // 2. Find by type match using reflection
    // 3. Try lazy creation via ServiceCreate interface
    // Panics if service not found
}
```

The application delegates to `ServiceManager`:
```go
type appImpl struct {
    services *ServiceManager
}

// Deprecated: use app.Services().Add(s)
func (app *appImpl) AddService(s Service) { app.services.Add(s) }
// Deprecated: use app.Services().Get(&v)
func (app *appImpl) GetService(v any)     { app.services.Get(v) }
// Preferred way to access services:
func (app *appImpl) Services() *ServiceManager { return app.services }
```

## Core Services

### 1. Configuration Service
**Type**: `Config` (type alias for `*config`)
**Implementation**: `launchr.ConfigFromFS`
**Purpose**: YAML-based configuration management with lazy creation via `ServiceCreate`

```go
// Config is a type alias, not an interface
type Config = *config

// Key methods:
func (cfg *config) Exists(key string) bool
func (cfg *config) Get(key string, v any) error
func (cfg *config) DirPath() string
func (cfg *config) Path(parts ...string) string
```

**Usage**:
```go
var config launchr.Config
app.Services().Get(&config)

var timeout time.Duration
config.Get("action.timeout", &timeout)
```

### 2. Action Manager Service
**Type**: `Manager` (type alias for `*actionManagerMap`)
**Implementation**: `action.NewManager` with lazy creation via `ServiceCreate`
**Purpose**: Action lifecycle management

```go
// Manager is a type alias, not an interface
type Manager = *actionManagerMap

// Key methods:
func (m *actionManagerMap) All() map[string]*Action
func (m *actionManagerMap) Get(id string) (*Action, bool)
func (m *actionManagerMap) Add(a *Action) error
func (m *actionManagerMap) Delete(id string)
func (m *actionManagerMap) AddDiscovery(fn DiscoverActionsFn)
func (m *actionManagerMap) Decorate(a *Action, withFns ...DecorateWithFn)
```

**Usage**:
```go
var manager action.Manager
app.Services().Get(&manager)

actions := manager.All()
```

### 3. Plugin Manager Service
**Type**: `PluginManager` (type alias for `pluginManagerMap`)
**Implementation**: `launchr.NewPluginManagerWithRegistered`
**Purpose**: Plugin management and discovery

```go
// PluginManager is a type alias, not an interface
type PluginManager = pluginManagerMap

// Methods:
func (m pluginManagerMap) ServiceInfo() ServiceInfo
func (m pluginManagerMap) All() PluginsMap

// GetPluginByType is a standalone generic function:
func GetPluginByType[T Plugin](mngr PluginManager) []MapItem[PluginInfo, T]
```

**Usage**:
```go
var pluginMgr launchr.PluginManager
app.Services().Get(&pluginMgr)

plugins := launchr.GetPluginByType[OnAppInitPlugin](pluginMgr)
```

## Service Registration

Services are registered during application initialization. Core services (`Config`, `Manager`)
use lazy creation via `ServiceCreate` interface - they are instantiated on first `Get()` call:

```go
func (app *appImpl) init() error {
    // Create ServiceManager
    app.services = launchr.NewServiceManager()
    app.pluginMngr = launchr.NewPluginManagerWithRegistered()

    // Register core services eagerly
    app.services.Add(app.mask)        // SensitiveMask
    app.services.Add(app.pluginMngr)  // PluginManager

    // Config and Manager are created lazily via ServiceCreate
    // when first requested with app.Services().Get(&config)
    return nil
}
```

## Service Usage Patterns

### In Plugins
```go
func (p *MyPlugin) OnAppInit(app launchr.App) error {
    // Get required services via ServiceManager
    var config launchr.Config
    app.Services().Get(&config)

    var manager action.Manager
    app.Services().Get(&manager)

    // Use services
    return p.initializeWithServices(config, manager)
}
```

### In Application Code
```go
func (app *appImpl) someMethod() error {
    var manager action.Manager
    app.Services().Get(&manager)

    return manager.SomeOperation()
}
```

## Service Lifecycle

### 1. Eager Registration Phase
Core services are registered during app initialization:
```go
app.services = launchr.NewServiceManager()
app.services.Add(app.mask)
app.services.Add(app.pluginMngr)
```

### 2. Lazy Creation Phase
Services implementing `ServiceCreate` are instantiated on first `Get()`:
```go
// Config implements ServiceCreate - created on first access
func (cfg *config) ServiceCreate(_ *ServiceManager) Service {
    return ConfigFromFS(os.DirFS("." + name))
}

// Manager implements ServiceCreate - created on first access
func (m *actionManagerMap) ServiceCreate(svc *ServiceManager) Service {
    var config Config
    svc.Get(&config)
    return NewManager(WithDefaultRuntime(config), ...)
}
```

### 3. Usage Phase
Services are retrieved when needed:
```go
var config launchr.Config
app.Services().Get(&config)
```

## Service Implementation Example

```go
type MyService struct {
    name string
    config Config
}

func (s *MyService) ServiceInfo() launchr.ServiceInfo {
    return launchr.ServiceInfo{}
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
// Use app.Services() to access ServiceManager
app.Services().Add(myService)

// Services with lazy creation implement ServiceCreate:
type MyService struct{}
func (s *MyService) ServiceInfo() launchr.ServiceInfo { return launchr.ServiceInfo{} }
func (s *MyService) ServiceCreate(svc *launchr.ServiceManager) launchr.Service {
    // Create service with dependencies
    return NewMyService()
}
```

### Service Retrieval
```go
// Services are retrieved by type via reflection.
// Panics if service not found - ensure service is registered.
var config launchr.Config
app.Services().Get(&config)
config.Get("key", &value)
```

## Strengths

1. **Type Safety**: Reflection-based but type-safe service retrieval
2. **Dependency Injection**: Clean separation of concerns
3. **Service Discovery**: Easy access to registered services
4. **Interface-Based**: Services defined by contracts, not implementations

## Limitations

1. **Reflection Overhead**: Runtime reflection for service retrieval
2. **Panic on Missing**: Missing services cause panics
3. **No Lifecycle Management**: Services don't have explicit lifecycle hooks (except `ServiceCreate` for lazy init)
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
Note: `ServiceCreate` interface already provides basic factory capability:
```go
type ServiceCreate interface {
    Service
    ServiceCreate(svc *ServiceManager) Service
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
    return launchr.ServiceInfo{}
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