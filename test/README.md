# Integration Tests

This directory contains integration tests, test scripts, and test helpers designed to provide comprehensive end-to-end
testing capabilities for the application.

## Overview

Our integration testing framework extends the standard [testscript](https://github.com/rogpeppe/go-internal)
functionality with custom commands and enhancements tailored to our specific testing needs. These tests validate the
complete application workflow, from binary execution to complex text processing scenarios.

## Documentation

For comprehensive information on testing methodologies, setup, and best practices, please refer to
our [testing documentation](../docs/development/test.md).

## Custom Testscript Commands

We have extended the standard testscript command set with several custom commands to enhance testing capabilities:

### Text Processing Commands

#### `txtproc` - Advanced Text Processing

Provides flexible text processing capabilities for manipulating test files and outputs.

**Available Operations:**

| Operation       | Description                         | Usage                                                                |
|-----------------|-------------------------------------|----------------------------------------------------------------------|
| `replace`       | Replace literal text strings        | `txtproc replace 'old_text' 'new_text' input.txt output.txt`         |
| `replace-regex` | Replace using regular expressions   | `txtproc replace-regex 'pattern' 'replacement' input.txt output.txt` |
| `remove-lines`  | Remove lines matching a pattern     | `txtproc remove-lines 'pattern' input.txt output.txt`                |
| `remove-regex`  | Remove text matching regex pattern  | `txtproc remove-regex 'pattern' input.txt output.txt`                |
| `extract-lines` | Extract lines matching a pattern    | `txtproc extract-lines 'pattern' input.txt output.txt`               |
| `extract-regex` | Extract text matching regex pattern | `txtproc extract-regex 'pattern' input.txt output.txt`               |

**Examples:**

```bash
# Replace version numbers in output
txtproc replace 'v1.0.0' 'v2.0.0' app_output.txt expected.txt

# Extract error messages using regex
txtproc extract-regex 'ERROR: .*' log.txt errors.txt

# Remove debug lines from output
txtproc remove-lines 'DEBUG:' verbose_output.txt clean_output.txt
```

### Utility Commands

#### `sleep` - Execution Delay

Pauses test execution for a specified duration, useful for timing-sensitive tests or waiting for asynchronous
operations.

**Usage:**

```bash
sleep <duration>
```

**Supported Duration Formats:**

- `ns` - nanoseconds
- `us` - microseconds
- `ms` - milliseconds
- `s` - seconds
- `m` - minutes
- `h` - hours

**Examples:**

```bash
# Wait for 1 second
sleep 1s

# Wait for 500 milliseconds
sleep 500ms

# Wait for 2 minutes
sleep 2m

# Wait for background process
sleep 100ms
```

#### `dlv` - Debug Integration

Runs the specified binary with [Delve](https://github.com/go-delve/delve) debugger support for interactive debugging
during tests.

**Usage:**

```bash
dlv <app_name>
```

**Prerequisites:**

- Binary must be compiled with debug headers (`-gcflags="all=-N -l"`)
- Delve must be installed in the testing environment

**Examples:**

```bash
# Debug the main application
dlv launchr
```

## Command Overrides

### Enhanced `kill` Command

We override the default testscript `kill` command to provide broader signal support beyond the standard implementation.

**Enhanced Features:**

- Support for additional POSIX signals
- Cross-platform signal handling
- Improved process termination reliability

**Usage:**

```bash
# Standard termination
kill bg-name

# Graceful shutdown with SIGTERM
kill -TERM bg-name
```

## Writing Integration Tests

### Basic Test Structure

Integration tests use the [txtar format](https://pkg.go.dev/github.com/rogpeppe/go-internal/txtar) to bundle test
scripts with their required files. See [examples](./testdata).

### Best Practices

#### Test Organization

- Group related tests in logical directories
- Use descriptive test file names
- Include both positive and negative test cases

#### File Management

- Keep test files small and focused
- Use meaningful fixture names
- Clean up temporary files when possible

#### Error Handling

- Test error conditions explicitly
- Verify error messages and exit codes
- Use `! exec` for commands expected to fail

#### Performance Considerations

- Use `sleep` judiciously to avoid slow tests
- Prefer deterministic waits over arbitrary delays
- Consider using `make test-short` for development

## Contributing

When adding new integration tests:

1. Follow the existing naming conventions
2. Include comprehensive test documentation
3. Test on multiple platforms when relevant
4. Add appropriate error handling
5. Update this README if adding new custom commands

## Related Resources

- [Testscript Documentation](https://github.com/rogpeppe/go-internal/tree/master/testscript)
- [Txtar Format Specification](https://pkg.go.dev/github.com/rogpeppe/go-internal/txtar)
- [Delve Debugger](https://github.com/go-delve/delve)
- [Testing Documentation](../docs/development/test.md)
