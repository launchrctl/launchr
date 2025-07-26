# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development Guidelines

**Documentation Updates**: When making code changes, ALWAYS update this documentation file to reflect:
- New architectural patterns or changes to existing ones
- Bug fixes with explanations of the root cause and solution
- New development patterns or conventions
- Changes to key interfaces or components
- Updates to build processes or commands

This ensures the documentation stays current and helps future developers understand the codebase evolution.

## Commands

### Build and Development
- `make` - Build the launchr binary to `bin/launchr`
- `make DEBUG=1` - Build with debug symbols for use with dlv debugger
- `make deps` - Fetch go dependencies
- `make test` - Run all tests
- `make lint` - Run golangci-lint with fixes
- `make install` - Install globally to `$GOPATH/bin`
- `go generate ./...` - Generate code (runs as part of build)

### Usage
- `bin/launchr --help` - Show help
- `bin/launchr --version` - Show version
- `bin/launchr build` - Build custom launchr with plugins

## Architecture Overview

Launchr is a CLI action runner that executes tasks defined in YAML files across multiple runtimes (containers, shell, plugins). The architecture is built around several core patterns:

### Core Systems

**Plugin Architecture**: Weight-based plugin system where plugins register via `init()` functions and implement lifecycle interfaces like `OnAppInitPlugin`, `CobraPlugin`, `DiscoveryPlugin`. Plugins are registered globally through `launchr.RegisterPlugin()`. 

**Plugin Hierarchies**: Plugins can have sub-plugins (module subpaths). During the build process, when checking for module replacements, the system must distinguish between a plugin and its sub-plugins. The fix ensures that exact path matches (`p.Path == repl`) are not skipped, only true subpath relationships (`p.Path != repl && strings.HasPrefix(p.Path, repl)`).

**Service-Oriented Design**: Core services (Config, Manager, PluginManager) are registered and retrieved through dependency injection via `App.AddService()` and `App.GetService()`. All services implement the `Service` interface.

**Runtime Strategy Pattern**: Multiple runtime implementations (shell, container, plugin) that implement the `Runtime` interface with `Init()`, `Execute()`, `Close()`, `Clone()` methods.

### Key Components

- **Action System** (`pkg/action/`): Core action entity with manager handling lifecycle, discovery, validation, and execution
- **Runtime System**: Shell, Container (Docker/K8s), and Plugin runtime implementations  
- **Discovery System**: YAML and embedded filesystem action discovery with extensible discovery plugins
- **Configuration System**: YAML-based config with dot-notation access and reflection-based caching
- **Plugin System** (`plugins/`): Core plugins for naming, CLI integration, discovery, value processing, and verbosity

### Important Interfaces

- `App`: Global application state management
- `Plugin`: Base plugin interface with `PluginInfo()` and lifecycle hooks
- `Service`: Dependency injection with `ServiceInfo()`
- `Runtime`: Action execution environment abstraction
- `Manager`: Action management and orchestration

### Key Files

- `app.go`: Main application implementation with plugin and service management
- `types.go`: Type aliases to reduce external dependencies
- `pkg/action/manager.go`: Action lifecycle management
- `pkg/action/action.go`: Core action entity
- `internal/launchr/config.go`: Configuration system
- `plugins/default.go`: Plugin registration

### Development Patterns

- Type aliases in `types.go` for clean interfaces
- Error handling with custom types and `errors.Is()` support
- Go template integration for dynamic action configuration
- Mutex-protected operations for concurrency safety
- `fs.FS` interface for filesystem abstraction
- JSON Schema validation for inputs and configuration
- **Plugin Replacement Logic**: In `plugins/builder/environment.go`, the system handles Go module replacements:
  - When ensuring modules are required, the system checks if a module is explicitly replaced (exact match) or if a plugin is a subpath of any replaced module (`p.Path != repl && strings.HasPrefix(p.Path, repl)`) to skip downloading its dependencies. This logic is inlined for direct use.
  - `ensureModuleRequired(ctx, modulePath, modReplace)`: This method ensures that a module is correctly added to `go.mod`, using a placeholder version if the module is replaced.

  This approach ensures that exact module replacements are handled correctly, while sub-plugins of replaced modules are properly skipped during dependency resolution, preventing unnecessary downloads and maintaining module integrity.

- **Environment Variable Handling**: Different runtimes handle environment variables differently:
  - **Shell Runtime** (`pkg/action/runtime.shell.go:47`): Automatically inherits all host environment variables using `append(os.Environ(), rt.Shell.Env...)`, making all host variables available to the script.
  - **Container Runtime** (`pkg/action/runtime.container.go:527`): Only passes explicitly defined environment variables from `runtime.env` in action YAML. Host environment variables must be explicitly referenced using `${VAR}` expansion syntax during action loading (`pkg/action/loader.go:59`).
  - **Environment Variable Expansion**: Uses `os.Expand()` to replace `${VAR}` patterns with host environment values during action loading, before container creation.
  - **EnvSlice Type** (`pkg/action/yaml.def.go:323`): Supports both YAML map (`KEY: value`) and array (`- KEY=value`) syntax for environment variable definitions.

### Execution Flow

1. Plugin registration and service initialization
2. Action discovery through registered discovery plugins
3. Cobra command generation from discovered actions
4. Multi-stage input validation (runtime flags, persistent flags, action parameters)
5. Runtime-specific execution with cleanup
6. Support for async action execution with status tracking

### Environment Variables

- `LAUNCHR_ACTIONS_PATH`: Path for action discovery (default: current directory)
