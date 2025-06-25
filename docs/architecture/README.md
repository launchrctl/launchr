# Architecture Documentation

This directory contains comprehensive architectural documentation for the Launchr project.

## Documents Overview

### [Architectural Patterns](ARCHITECTURAL_PATTERNS.md)
Comprehensive analysis of all architectural patterns used in Launchr, including:
- **Design Patterns**: Factory, Strategy, Decorator, Observer, Repository, etc.
- **Anti-Patterns**: Identified issues and code smells
- **Improvement Suggestions**: Detailed recommendations with priorities
- **Implementation Examples**: Code samples showing pattern usage

### [Logging Architecture](LOGGING_ARCHITECTURE.md)
Deep dive into Launchr's sophisticated dual logging system:
- **Internal Logging (`Log()`)**: Developer-focused structured logging
- **Terminal Logging (`Term()`)**: User-facing styled output
- **Best Practices**: When to use each system
- **Configuration**: Runtime setup and customization

### [Plugin System](PLUGIN_SYSTEM.md)
Complete guide to Launchr's extensible plugin architecture:
- **Plugin Interfaces**: Base and specialized plugin types
- **Registration System**: Weight-based plugin ordering
- **Core Plugins**: Built-in plugin functionality
- **Development Guide**: How to create custom plugins

### [Service System](SERVICE_SYSTEM.md)
Documentation of the dependency injection and service management:
- **Service Container**: Type-safe service registration and retrieval
- **Core Services**: Config, Manager, PluginManager
- **Service Lifecycle**: Creation, registration, and usage
- **Best Practices**: Service design and implementation

## Architecture Overview

Launchr is built around several key architectural principles:

### 1. **Plugin-Based Extensibility**
- Weight-based plugin system with lifecycle hooks
- Interface-driven design for maximum flexibility
- Clear separation between core and plugin functionality

### 2. **Service-Oriented Architecture**
- Dependency injection through service container
- Type-safe service resolution using reflection
- Clean separation of concerns between services

### 3. **Dual Logging System**
- Internal structured logging for developers (`Log()`)
- Styled terminal output for users (`Term()`)
- Runtime configuration and level management

### 4. **Strategy Pattern for Runtimes**
- Multiple execution environments (shell, container, plugin)
- Pluggable runtime implementations
- Consistent lifecycle management across runtimes

### 5. **Repository Pattern for Actions**
- Centralized action management and discovery
- CRUD operations with validation
- Multiple discovery strategies (filesystem, embedded)

## Key Strengths

✅ **Excellent Extensibility**: Plugin architecture allows easy feature addition
✅ **Clean Separation**: Clear boundaries between internal and user-facing code
✅ **Type Safety**: Reflection-based but type-safe service resolution
✅ **User Experience**: Sophisticated terminal output with styling
✅ **Developer Experience**: Structured logging with contextual information

## Areas for Improvement

⚠️ **Error Handling**: Replace panics with proper error returns
⚠️ **Thread Safety**: Add synchronization to terminal logging
⚠️ **Reflection Usage**: Consider compile-time alternatives
⚠️ **Interface Segregation**: Split large interfaces into focused ones
⚠️ **Plugin Dependencies**: Implement proper dependency resolution

## Reading Guide

1. **New Developers**: Start with [Plugin System](PLUGIN_SYSTEM.md) and [Service System](SERVICE_SYSTEM.md)
2. **Architecture Review**: Focus on [Architectural Patterns](ARCHITECTURAL_PATTERNS.md)
3. **Logging Implementation**: See [Logging Architecture](LOGGING_ARCHITECTURE.md)
4. **Contributing**: Review all documents for complete understanding

## Relationship to Other Documentation

This architecture documentation complements:
- **[../development/](../development/)**: Development guides and practices
- **[../actions.md](../actions.md)**: Action definition and usage
- **[../config.md](../config.md)**: Configuration management
- **[CLAUDE.md](../../CLAUDE.md)**: AI assistant guidance

## Maintenance

These documents should be updated when:
- New architectural patterns are introduced
- Existing patterns are significantly modified
- Major refactoring affects system architecture
- New services or plugins are added to the core system

For questions or suggestions about the architecture, please create an issue or discussion in the project repository.