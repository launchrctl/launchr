# Application Testing Guide

This document provides comprehensive information about testing methodologies, tools, and procedures for the application.

## Table of Contents

- [Go Testing Solution](#go-testing-solution)
- [Running Tests](#running-tests)
- [Code Quality & Linting](#code-quality--linting)
- [Local Cross-Platform Testing](#local-cross-platform-testing)
- [GitHub Actions Debug Workflow](#github-actions-debug-workflow)
- [Reusable GitHub Workflows](#reusable-github-workflows)

## Go Testing Solution

Our testing strategy combines unit tests with integration testing to ensure comprehensive coverage and reliability.

### Unit Testing

Most of the codebase is covered with standard Go tests, providing fast feedback during development.

### Test Output Formatting

We use **gotestfmt** to provide clean, readable test output with improved formatting and colored output. This tool
transforms the standard Go test output into a more user-friendly format, making it easier to identify failures and
understand test results at a glance.

### Integration Testing

We use **Testscript** for integration testing of the built binary, offering robust end-to-end test capabilities.

#### Why Testscript?

- **Mature and Reliable**: Based on the [Go standard library's
  `cmd/go/internal/script`](https://cs.opensource.google/go/go/+/refs/tags/go1.22.0:src/cmd/go/internal/script/)
- **Declarative Scripting**: Uses a simple scripting language in `.txt` files for robust end-to-end tests
- **Self-Contained**: Utilizes the [txtar format](https://pkg.go.dev/github.com/rogpeppe/go-internal/txtar) to bundle
  test scripts and fixture files
- **Portable**: Tests are easy to manage and transfer between environments

**Resources:**

- [Testscript Project](https://github.com/rogpeppe/go-internal)
- [Txtar Format Documentation](https://pkg.go.dev/github.com/rogpeppe/go-internal/txtar)

## Running Tests

### Basic Test Commands

```bash
# Run all tests with verbose output
go test -v ./...

# Run a specific test function
go test -v -run TestFunctionName ./...

# Run tests with coverage report
go test -v -cover ./...
```

### Makefile Targets

We provide convenient Makefile targets for different testing scenarios:

```bash
# Run full test suite
make test

# Run short tests (skips heavy/slow tests)
make test-short
```

**Note:** Use `make test-short` during development to skip time-consuming tests and get faster feedback.

## Code Quality & Linting

### Linting Standards

The codebase is linted using **golangci-lint** to maintain consistent code quality and style.

#### Running the Linter

```bash
# Lint the entire codebase
make lint
```

#### Guidelines

- Follow all standard linting rules
- Use `//nolint` comments only in exceptional cases
- Ensure all code passes linting checks before submitting

## Local Cross-Platform Testing

Testing across different operating systems is crucial for ensuring compatibility. Here's how to set up local
cross-platform testing environments:

### Platform-Specific Solutions

| Host OS     | Target OS   | Recommended Solution                                                                    |
|-------------|-------------|-----------------------------------------------------------------------------------------|
| **Linux**   | Windows     | [dockur/windows](https://github.com/dockur/windows) Docker container                    |
| **Linux**   | macOS       | [dockur/macos](https://github.com/dockur/macos) Docker container                        |
| **Linux**   | Other Linux | [Lima](https://lima-vm.io/)                                                             |
| **macOS**   | Windows     | [UTM](https://mac.getutm.app/) (free) or [Parallels](https://www.parallels.com/) (paid) |
| **macOS**   | Linux       | [Lima](https://lima-vm.io/)                                                             |
| **Windows** | Linux       | [WSL2](https://docs.microsoft.com/en-us/windows/wsl/)                                   |
| **Windows** | macOS       | [dockur/macos](https://github.com/dockur/macos) via Docker                              |

### Quick Setup Examples

```bash
# Linux testing Windows via Docker
docker run -it dockur/windows

# macOS/Linux testing via Lima
lima create --name test-env ubuntu
lima start test-env
lima shell test-env
```

## GitHub Actions Debug Workflow

When local testing isn't sufficient, you can debug and test directly on GitHub's infrastructure using our debug
workflow.

### Setup Instructions

1. **Fork the Repository**
    - Provides full control over the repository
    - Maintains clean GitHub Actions history for your project

2. **Access the Debug Workflow**
    - Navigate to the **Actions** tab in your forked repository
    - Select **"🚧 Debug with SSH"** workflow
    - Click **"Run workflow"** and select:
        - Target branch
        - Operating system (Windows/macOS/Linux)
        - Architecture (amd64/arm64)

3. **Connect to the Runner**
    - Open the new pipeline execution
    - Click on the **"👉 How to connect 👈"** job
    - Wait for dependency installation to complete
    - Find the SSH connection command in the logs

### VS Code Integration

For a full development environment, you can connect VS Code to the GitHub runner:

#### Prerequisites

- Install the **"Remote - SSH"** extension in VS Code

#### Connection Steps

1. Open Command Palette (`Ctrl+Shift+P` / `Cmd+Shift+P`)
2. Select **"Remote-SSH: Add New SSH Host..."**
3. Enter the SSH command from the GitHub Actions logs
4. Click **"Connect"** in the popup or use **"Remote-SSH: Connect to Host..."**
5. Open the project path specified in the logs

### Use Cases

- **Remote Debugging**: Forward `dlv` port for Go debugging
- **Cross-Platform Testing**: Test on different OS/architecture combinations
- **CI/CD Troubleshooting**: Debug failing workflows in the actual environment
- **Development**: Full development environment without local setup

## Reusable GitHub Workflows

To maintain consistency across launchr-related projects, we provide reusable workflows that can be integrated into other
repositories.

### Integration Example

Create or update your `.github/workflows/ci.yaml` file:

```yaml
name: 🧪 Code Coverage & Testing

on:
  push:
    branches:
      - '**'
    paths-ignore:
      - 'README.md'
      - 'LICENSE'
      - '.gitignore'
      - 'example/**'
      - 'docs/**'

concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: true

jobs:
  tests:
    name: 🛡️ Multi-Platform Testing Suite
    uses: launchr/launchr/.github/workflows/test-suite.yaml@main
```

### Benefits of Reusable Workflows

- **Consistency**: Standardized testing across all projects
- **Maintenance**: Centralized updates and improvements
- **Efficiency**: Reduced duplication of workflow configuration
- **Best Practices**: Automatically includes optimized testing strategies

## Best Practices

### Development Workflow

1. Write tests alongside your code
2. Use `make test-short` during development for quick feedback
3. Run full test suite before committing: `make test`
4. Ensure linting passes: `make lint`
5. Test cross-platform compatibility when needed

### CI/CD Integration

- Use the reusable workflow for consistent testing
- Configure appropriate triggers and path ignores
- Monitor test coverage and maintain quality standards

### Debugging Strategy

1. Start with local testing
2. Use cross-platform VMs for OS-specific issues
3. Leverage GitHub Actions debug workflow for CI/CD issues
4. Utilize VS Code remote development for complex debugging

---

For questions or contributions to the testing infrastructure, please refer to the project's contribution guidelines.
