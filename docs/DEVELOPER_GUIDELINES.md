# Developer Guidelines

This document provides comprehensive guidelines for developers working on the Launchr project.

## Table of Contents

1. [Getting Started](#getting-started)
2. [Code Style and Conventions](#code-style-and-conventions)
3. [Architecture Guidelines](#architecture-guidelines)
4. [Logging Guidelines](#logging-guidelines)
5. [Plugin Development](#plugin-development)
6. [Service Development](#service-development)
7. [Testing Guidelines](#testing-guidelines)
8. [Error Handling](#error-handling)
9. [Performance Considerations](#performance-considerations)
10. [Contributing Guidelines](#contributing-guidelines)

## Getting Started

### Prerequisites
- Go 1.24 or later
- Make
- Docker (for container runtime testing)
- Kubernetes (optional, for k8s runtime testing)

### Development Setup
```bash
# Clone the repository
git clone <repository-url>
cd launchr

# Install dependencies
make deps

# Build the project
make

# Run tests
make test

# Run linter
make lint
```

### Development Environment
```bash
# Build with debug symbols for debugging
make DEBUG=1

# Run with verbose logging
LAUNCHR_LOG_LEVEL=DEBUG bin/launchr --help
```

## Code Style and Conventions

### Go Code Style
Follow standard Go conventions as defined by `gofmt` and `golangci-lint`:

```go
// Good: Clear, descriptive function names
func NewActionManager(config Config) *ActionManager {
    return &ActionManager{
        config: config,
        actions: make(map[string]*Action),
    }
}

// Bad: Unclear, abbreviated names
func NewActMgr(cfg Cfg) *ActMgr {
    return &ActMgr{cfg: cfg, acts: make(map[string]*Act)}
}
```

### File Organization
```
pkg/
├── action/          # Action-related functionality
├── driver/          # Runtime drivers (docker, k8s)
├── jsonschema/      # JSON schema utilities
└── archive/         # Archive utilities

internal/
└── launchr/         # Internal application code

plugins/
├── actionnaming/    # Action naming plugin
├── actionscobra/    # Cobra integration plugin
└── ...

cmd/
└── launchr/         # Command-line entry point
```

### Import Organization
```go
package example

import (
    // Standard library
    "context"
    "fmt"
    "io"
    
    // Third-party dependencies
    "github.com/spf13/cobra"
    "gopkg.in/yaml.v3"
    
    // Local imports
    "github.com/launchrctl/launchr/internal/launchr"
    "github.com/launchrctl/launchr/pkg/action"
)
```

### Naming Conventions
- **Packages**: Short, single-word, lowercase (e.g., `action`, `driver`)
- **Interfaces**: Noun or adjective ending in -er (e.g., `Manager`, `Runner`)
- **Functions**: Descriptive verbs (e.g., `CreateAction`, `ValidateInput`)
- **Constants**: CamelCase with descriptive names
- **Variables**: CamelCase, descriptive but concise

## Architecture Guidelines

### Plugin Development Rules

1. **Single Responsibility**: Each plugin should have one clear purpose
2. **Minimal Dependencies**: Avoid tight coupling between plugins
3. **Weight Selection**: Choose weights thoughtfully for proper ordering
4. **Interface Implementation**: Only implement interfaces you actually use

```go
// Good: Focused plugin with single responsibility
type ActionNamingPlugin struct {
    transformer NameTransformer
}

func (p *ActionNamingPlugin) PluginInfo() launchr.PluginInfo {
    return launchr.PluginInfo{
        Name:   "action-naming",
        Weight: 100,
    }
}

// Bad: Plugin trying to do everything
type MegaPlugin struct{}
func (p *MegaPlugin) OnAppInit(...) error { /* complex logic */ }
func (p *MegaPlugin) CobraAddCommands(...) error { /* more logic */ }
func (p *MegaPlugin) DiscoverActions(...) error { /* even more logic */ }
```

### Service Design Rules

1. **Interface Segregation**: Keep interfaces focused and small
2. **Dependency Injection**: Use constructor injection for dependencies
3. **Service Lifecycle**: Consider service startup and shutdown
4. **Error Handling**: Return errors, don't panic

```go
// Good: Small, focused interface
type ActionReader interface {
    Get(id string) (*Action, bool)
    All() map[string]*Action
}

type ActionWriter interface {
    Add(*Action) error
    Delete(id string) error
}

// Bad: Large interface with many responsibilities
type ActionEverything interface {
    Get(id string) (*Action, bool)
    Add(*Action) error
    Delete(id string) error
    Validate(*Action) error
    Execute(*Action) error
    // ... 10 more methods
}
```

## Logging Guidelines

### Use the Right Logger

#### Internal Logging (`Log()`) - For Developers
```go
// Good: Debug information for developers
Log().Debug("initialising application", "config_dir", app.cfgDir)
Log().Error("error on plugin init", "plugin", p.Name, "err", err)

// Bad: User-facing messages in internal log
Log().Info("Build completed successfully")  // Users won't see this
```

#### Terminal Logging (`Term()`) - For Users
```go
// Good: User-facing status messages
Term().Info().Printfln("Starting to build %s", buildName)
Term().Warning().Printfln("Configuration file not found, using defaults")
Term().Success().Println("Build completed successfully")

// Bad: Debug information in terminal output
Term().Info().Printf("Debug: variable state = %+v", internalState)
```

### Structured Logging Best Practices
```go
// Good: Structured with context
Log().Error("failed to execute action", 
    "action_id", action.ID(),
    "runtime", action.Runtime().Type(),
    "error", err)

// Bad: String concatenation
Log().Error("failed to execute action " + action.ID() + ": " + err.Error())
```

## Plugin Development

### Plugin Lifecycle

1. **Registration**: Plugins register themselves in `init()`
2. **Initialization**: Implement required interfaces
3. **Execution**: Plugin methods called by the system

### Plugin Template
```go
package myplugin

import "github.com/launchrctl/launchr"

type MyPlugin struct {
    name string
}

func (p *MyPlugin) PluginInfo() launchr.PluginInfo {
    return launchr.PluginInfo{
        Name:   p.name,
        Weight: 100, // Choose appropriate weight
    }
}

func (p *MyPlugin) OnAppInit(app launchr.App) error {
    // Get required services
    var config launchr.Config
    app.GetService(&config)
    
    // Initialize plugin
    return p.initialize(config)
}

func (p *MyPlugin) initialize(config launchr.Config) error {
    // Plugin-specific initialization
    return nil
}

func init() {
    launchr.RegisterPlugin(&MyPlugin{name: "my-plugin"})
}
```

### Plugin Testing
```go
func TestMyPlugin(t *testing.T) {
    plugin := &MyPlugin{name: "test-plugin"}
    
    // Test plugin info
    info := plugin.PluginInfo()
    assert.Equal(t, "test-plugin", info.Name)
    
    // Test initialization
    app := createTestApp()
    err := plugin.OnAppInit(app)
    assert.NoError(t, err)
}
```

## Service Development

### Service Implementation
```go
type MyService struct {
    config Config
    logger *Logger
}

func (s *MyService) ServiceInfo() launchr.ServiceInfo {
    return launchr.ServiceInfo{
        Name:        "my-service",
        Description: "Example service for demonstration",
    }
}

func (s *MyService) Initialize() error {
    // Service initialization logic
    return nil
}

func NewMyService(config Config, logger *Logger) *MyService {
    return &MyService{
        config: config,
        logger: logger,
    }
}
```

### Service Testing
```go
func TestMyService(t *testing.T) {
    config := createTestConfig()
    logger := createTestLogger()
    
    service := NewMyService(config, logger)
    
    err := service.Initialize()
    assert.NoError(t, err)
    
    info := service.ServiceInfo()
    assert.Equal(t, "my-service", info.Name)
}
```

## Testing Guidelines

### Test Structure
```go
func TestFunctionName(t *testing.T) {
    // Arrange
    input := createTestInput()
    expected := expectedResult()
    
    // Act
    result, err := FunctionUnderTest(input)
    
    // Assert
    assert.NoError(t, err)
    assert.Equal(t, expected, result)
}
```

### Test Helpers
```go
func createTestApp() launchr.App {
    app := &testApp{
        services: make(map[launchr.ServiceInfo]launchr.Service),
    }
    return app
}

func createTestConfig() launchr.Config {
    return &testConfig{
        data: make(map[string]interface{}),
    }
}
```

### Table-Driven Tests
```go
func TestActionValidation(t *testing.T) {
    tests := []struct {
        name    string
        action  *Action
        wantErr bool
    }{
        {
            name:    "valid action",
            action:  createValidAction(),
            wantErr: false,
        },
        {
            name:    "invalid action - missing ID",
            action:  createActionWithoutID(),
            wantErr: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidateAction(tt.action)
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

## Error Handling

### Error Creation
```go
// Good: Descriptive error with context
func (m *Manager) GetAction(id string) (*Action, error) {
    action, exists := m.actions[id]
    if !exists {
        return nil, fmt.Errorf("action %q not found", id)
    }
    return action, nil
}

// Bad: Generic error
func (m *Manager) GetAction(id string) (*Action, error) {
    action, exists := m.actions[id]
    if !exists {
        return nil, errors.New("not found")
    }
    return action, nil
}
```

### Error Wrapping
```go
func (r *Runtime) Execute(ctx context.Context, action *Action) error {
    if err := r.prepare(ctx, action); err != nil {
        return fmt.Errorf("failed to prepare runtime: %w", err)
    }
    
    if err := r.run(ctx, action); err != nil {
        return fmt.Errorf("failed to execute action: %w", err)
    }
    
    return nil
}
```

### Avoid Panics
```go
// Good: Return error
func RegisterPlugin(p Plugin) error {
    info := p.PluginInfo()
    if _, exists := registeredPlugins[info]; exists {
        return fmt.Errorf("plugin %q already registered", info.Name)
    }
    registeredPlugins[info] = p
    return nil
}

// Bad: Panic (current implementation - should be changed)
func RegisterPlugin(p Plugin) {
    info := p.PluginInfo()
    if _, exists := registeredPlugins[info]; exists {
        panic(fmt.Errorf("plugin %q already registered", info.Name))
    }
    registeredPlugins[info] = p
}
```

## Performance Considerations

### Minimize Reflection
```go
// Good: Direct type assertion
func GetService[T Service](container ServiceContainer) (T, error) {
    var zero T
    service, exists := container.GetByType(reflect.TypeOf(zero))
    if !exists {
        return zero, fmt.Errorf("service %T not found", zero)
    }
    return service.(T), nil
}

// Current: Heavy reflection usage
func (app *appImpl) GetService(v any) {
    // Complex reflection logic...
}
```

### Efficient Logging
```go
// Good: Check log level before expensive operations
if Log().Level() >= LogLevelDebug {
    Log().Debug("complex operation result", "data", expensiveSerialize(data))
}

// Bad: Always compute expensive data
Log().Debug("complex operation result", "data", expensiveSerialize(data))
```

### Resource Management
```go
// Good: Proper cleanup with defer
func (r *Runtime) Execute(ctx context.Context) error {
    resource, err := r.acquireResource()
    if err != nil {
        return err
    }
    defer func() {
        if cleanupErr := resource.Close(); cleanupErr != nil {
            Log().Error("failed to cleanup resource", "error", cleanupErr)
        }
    }()
    
    return r.doWork(resource)
}
```

## Contributing Guidelines

### Before Making Changes

1. **Read Architecture Documentation**: Understand the system design
2. **Check Existing Issues**: Look for related work or discussions
3. **Write Tests**: Ensure your changes are well-tested
4. **Run Linter**: `make lint` must pass
5. **Update Documentation**: Keep docs in sync with code changes

### Pull Request Process

1. **Feature Branch**: Create a descriptive branch name
2. **Small Commits**: Make logical, focused commits
3. **Clear Messages**: Write descriptive commit messages
4. **Update Tests**: Add or modify tests as needed
5. **Documentation**: Update relevant documentation

### Code Review Checklist

- [ ] Code follows style guidelines
- [ ] All tests pass
- [ ] Linter passes without warnings
- [ ] Changes are well-documented
- [ ] No breaking changes without version bump
- [ ] Error handling is appropriate
- [ ] Logging follows guidelines
- [ ] Performance impact considered

### Release Process

1. **Version Bump**: Update version in appropriate files
2. **Changelog**: Update CHANGELOG.md with changes
3. **Tag Release**: Create annotated git tag
4. **GitHub Release**: Create release with binaries

## Best Practices Summary

### DO ✅
- Use structured logging with context
- Implement focused interfaces
- Handle errors gracefully
- Write comprehensive tests
- Document public APIs
- Follow Go conventions
- Use appropriate logging system
- Minimize reflection usage
- Clean up resources properly

### DON'T ❌
- Use panics for recoverable errors
- Create large, monolithic interfaces
- Mix logging systems inappropriately
- Ignore test coverage
- Break existing APIs without versioning
- Use magic numbers without constants
- Leak resources or goroutines
- Write untestable code

## Getting Help

- **Architecture Questions**: Review `docs/architecture/`
- **Plugin Development**: See `docs/development/plugin.md`
- **Service Development**: See `docs/development/service.md`
- **General Questions**: Create a GitHub issue
- **Discussions**: Use GitHub Discussions for broader topics

## Useful Commands

```bash
# Development
make deps          # Install dependencies
make              # Build project
make DEBUG=1      # Build with debug symbols
make test         # Run tests
make lint         # Run linter

# Testing
go test ./...                    # Run all tests
go test -v ./pkg/action/...     # Run specific package tests
go test -race ./...             # Run tests with race detection
go test -cover ./...            # Run tests with coverage

# Debugging
dlv debug ./cmd/launchr         # Debug with Delve
LAUNCHR_LOG_LEVEL=DEBUG ./bin/launchr  # Verbose logging

# Building
make install      # Install globally
make clean        # Clean build artifacts
```

Remember: The goal is to maintain high code quality while preserving the excellent architectural foundation that already exists in Launchr.