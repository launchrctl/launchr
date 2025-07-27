# Dual Logging Architecture Analysis

## Overview

Launchr implements a sophisticated **dual logging architecture** that separates internal debugging/diagnostics from user-facing terminal output. This is an advanced architectural pattern that provides excellent separation of concerns between developer logging and user communication.

## The Two Logging Systems

### 1. Internal Logging System: `Log()`

**Location**: `internal/launchr/log.go`
**Purpose**: Developer-focused structured logging for debugging and diagnostics
**Technology**: Go's structured logger (`slog`) with pterm integration

#### Key Features:
- **Structured Logging**: Uses `slog` for key-value structured logs
- **Multiple Handlers**: Console, TextHandler, JSON Handler support
- **Configurable Levels**: Disabled, Debug, Info, Warn, Error
- **Thread-Safe**: Uses atomic pointer for default logger
- **Default Behavior**: Outputs to `io.Discard` (silent by default)
- **Runtime Configuration**: Log level and output can be changed at runtime

#### Implementation Details:
```go
// Global atomic logger for thread safety
var defaultLogger atomic.Pointer[Logger]

// Logger wraps slog with additional options
type Logger struct {
    *Slog  // Go's structured logger
    LogOptions
}

// Usage pattern
Log().Debug("shutdown cleanup")
Log().Error("error on OnAppInit", "plugin", p.K.String(), "err", err)
```

#### Usage Statistics:
- **18 instances** across the codebase
- Primarily used in:
  - Application lifecycle (`app.go`)
  - Action management (`pkg/action/`)
  - Driver implementations (`pkg/driver/`)
  - Plugin systems (`plugins/`)

### 2. Terminal/User Logging System: `Term()`

**Location**: `internal/launchr/term.go`
**Purpose**: User-facing formatted terminal output with styling and colors
**Technology**: pterm (Pretty Terminal) library

#### Key Features:
- **Styled Output**: Colored, formatted terminal output with prefixes
- **Multiple Printers**: Basic, Info, Warning, Success, Error
- **User-Focused**: Designed for end-user communication
- **Global Control**: Can be enabled/disabled globally
- **Reflection-Based**: Uses reflection for pterm WithWriter method calls

#### Implementation Details:
```go
// Terminal with styled printers
type Terminal struct {
    w io.Writer     // Target writer
    p []TextPrinter // Styled printers array
    enabled bool    // Global enable/disable
}

// Usage patterns
Term().Info().Printfln("Starting to build %s", b.PkgName)
Term().Warning().Printfln("Error on application shutdown cleanup:\n %s", err)
Term().Error().Println(err)
```

#### Usage Statistics:
- **7 instances** across the codebase
- Primarily used in:
  - Application error handling (`app.go`)
  - Build system (`plugins/builder/`)
  - User-facing operations

## Best Practices for Usage

### When to Use Internal Logging (`Log()`)
- Debug information for developers
- Error diagnostics with context
- Performance metrics and traces  
- Internal state changes
- Plugin lifecycle events

```go
// Good examples
Log().Debug("initialising application")
Log().Error("error on OnAppInit", "plugin", p.K.String(), "err", err)
Log().Debug("executing shell", "cmd", cmd)
```

### When to Use Terminal Logging (`Term()`)
- User-facing status messages
- Error messages users need to see
- Build progress information
- Success/completion notifications
- Warnings about user actions

```go
// Good examples  
Term().Info().Printfln("Starting to build %s", name)
Term().Warning().Printfln("Error on application shutdown cleanup:\n %s", err)
Term().Success().Println("Build completed successfully")
```

### Anti-Patterns to Avoid
```go
// DON'T: Use internal logging for user messages
Log().Info("Build started")  // Users won't see this

// DON'T: Use terminal logging for debug info
Term().Info().Printf("Debug: variable x = %v", x)  // Clutters user output

// DON'T: Mix logging systems inconsistently
Log().Error("Failed to start")
Term().Error().Println("Failed to start")  // Pick one based on audience
```

## Configuration

### Internal Logging Configuration:
```go
// Runtime level changes
Log().SetLevel(LogLevelDebug)
Log().SetOutput(os.Stderr)

// Different handler types
NewConsoleLogger(w)     // Pretty console output
NewTextHandlerLogger(w) // Plain text output  
NewJSONHandlerLogger(w) // JSON structured output
```

### Terminal Configuration:
```go
// Global enable/disable
Term().EnableOutput()
Term().DisableOutput()

// Output redirection
Term().SetOutput(writer)

// Individual printer access
Term().Info().Println("message")
Term().Warning().Printf("format %s", arg)
```

## Thread Safety

- **Internal Logging**: ✅ Thread-safe using `atomic.Pointer[Logger]`
- **Terminal Logging**: ⚠️ Global instance without explicit synchronization

## Performance Considerations

- **Internal Logging**: Optimized with `io.Discard` by default
- **Terminal Logging**: Uses reflection in `SetOutput()` which has overhead