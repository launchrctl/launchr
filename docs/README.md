# Launchr Documentation

## User Documentation
1. [Actions](actions.md) - Action definition and usage
2. [Actions Schema](actions.schema.md) - Schema validation for actions
3. [Global Configuration](config.md) - Application configuration
4. [Built-in functionality](#built-in-functionality)

## Developer Documentation
5. **[Developer Guidelines](DEVELOPER_GUIDELINES.md)** - Comprehensive development guide
6. **[Architecture](architecture/)** - System architecture documentation
   - [Architectural Patterns](architecture/ARCHITECTURAL_PATTERNS.md)
   - [Logging Architecture](architecture/LOGGING_ARCHITECTURE.md)
   - [Plugin System](architecture/PLUGIN_SYSTEM.md)
   - [Service System](architecture/SERVICE_SYSTEM.md)
7. [Development](development) - Specific development topics
   - [Plugin Development](development/plugin.md)
   - [Service Development](development/service.md)

## Build plugin

There are the following build options:
1. `-o, --output OUTPUT` - result file. If empty, application name is used.
2. `-n, --name NAME` - application name.
3. `-p, --plugin PLUGIN[@v1.1]` - use plugin in the built launchr. The flag may be specified multiple times.
    ```shell
    launchr build \
      -p github.com/launchrctl/launchr \ 
      -p github.com/launchrctl/launchr@v0.1.0
    ```
4. `-r, --replace OLD=NEW` - replace go dependency, see [go mod edit -replace](https://go.dev/ref/mod#go-mod-edit). The flag may be specified multiple times.

   The directive may be used to replace a private repository with a local path or to set a specific version of a module. Example:
    ```shell
    launchr build --replace github.com/launchrctl/launchr=/path/to/local/dir
    launchr build --replace github.com/launchrctl/launchr=github.com/launchrctl/launchr@v0.2.0
    ```

5. `-d, --debug` - include debug flags into the build to support go debugging like [Delve](https://github.com/go-delve/delve).
   Without the flag, all debugging info is trimmed.
6. `-h, --help` - output help message
