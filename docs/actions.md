# Actions

Actions give an ability to run arbitrary commands in containers.

## Supported runtimes

  * Container runtimes (docker, kubernetes)
  * Shell (host)
  * Go language (launchr plugin)

## Supported container runtimes

  * `docker` (default)
  * `kubernetes`

Container runtime is configured globally for all actions of type container. See [Global Configuration](config.md#container-runtime) definition.  
If not specified, the action will use **Docker** as a default runtime.

Docker or Kubernetes are not required to be installed to use containers. But the configuration must be present.

### Docker

Runtime tries to connect using the following configuration:

1. `unix:///run/docker.sock` - default docker socket path
2. Via [Docker environment variables](https://docs.docker.com/reference/cli/docker/#environment-variables), see the following variables: 
   * `DOCKER_HOST`
   * `DOCKER_API_VERSION`
   * `DOCKER_CERT_PATH`
   * `DOCKER_TLS_VERIFY`

Normally, if the Docker installed locally, `unix:///run/docker.sock` will be present and used by default.  
**NB!** If the docker is not running locally, you may have incorrect mounting of the paths. Consider using a flag `--remote-runtime`.

### Kubernetes

Runtime tries to connect using the following configuration:

1. `~/.kube/config` - default kubectl configuration directory.
2. `KUBECONFIG` environment variable, see [kuberenetes documentation](https://kubernetes.io/docs/concepts/configuration/organize-cluster-access-kubeconfig/#the-kubeconfig-environment-variable)
    Usage example: `export KUBECONFIG="/etc/rancher/k3s/k3s.yaml"`

## Action definition

Action configuration files are written in `yaml`, example declaration:
```yaml
action:
  title: Verb
  description: Handles some logic
  alias:
    - "alias1"
    - "alias2"
  arguments:
    - name: myArg1
      title: Argument 1
      description: Some additional info for arg
    - name: myArg2
      title: Argument 2
      description: Some additional info for arg
  options:
    - name: optStr
      title: Option string
      description: Some additional info for option

runtime:
  type: container
  image: python:3.7-slim
  command:
    - python3
    - {{ .myArg1 }} {{ .myArg2 }}
    - {{ .optStr }}
    - ${ENV_VAR}
```

For more detailed specification and examples of action definition, see [actions.schema.md](actions.schema.md)

## Actions discovery

The action files must preserve a tree structure like `**/**/actions/*/action.yaml` or `actions/*/action.yaml` to be discovered.  
Example:
```
actions:
└── foo
    └── action.yaml
foo
└── bar
    └── buz
        └── actions
            └── waldo
                └── action.yaml
bar
└── actions
    ├── foo
    │   ├── build.sh
    │   └── action.yaml
    └── buz
        ├── bump.py
        └── action.yaml
```

The example structure produces the following commands:

```shell
$ launchr --help
launchr is a versatile CLI action runner that executes tasks defined in local or embeded yaml files across multiple runtimes
...
Actions:
  foo                foo: foo description
  foo.bar.buz:waldo  foo bar buz waldo: foo.bar.buz:waldo description
  bar:foo            bar foo: bar:foo description
  bar:buz            bar buz: bar:buz description
...
```

### Action execution

To run the command simply run:
```shell
$ launchr foo:bar [args...] [--options...]
```

To get more help for the action:


```shell
$ launchr foo:bar --help

Foo Bar: foo:bar description

Usage:
  launchr foo:bar argStr [argInt] [flags]

Arguments:
      argInt int      Argument Integer: This is an optional integer argument
      argStr string   Argument String: This is a required implicit string argument

Options:
      --optArray strings     Option Array String: This is an optional array<string> option with a default value
      --optArrayBool bools   Option Array Boolean: This is an optional enum<boolean> option with a default value (default [])
      --optBoolean           Option Boolean: This is an optional boolean option with a default value
      --optEnum string       Option Enum: This is an optional enum<string> option with a default value (default "enum1")
      --optIP string         Option String IP: This is an optional string option with a format validation (ipv4) (default "1.1.1.1")
      --optInteger int       Option Integer: This is an optional boolean option with a default value
      --optNumber float      Option Number: This is an optional number option with a default value (default 3.14)
      --optString string     Option String: This is an optional string option with a default value

Action runtime options:
      --entrypoint string   Image Entrypoint: Overwrite the default ENTRYPOINT of the image. Example: --entrypoint "/bin/sh"
      --exec                Exec command: Overwrite the command of the action. Argument and options are not validated, sets container CMD directly. Example usage: --exec -- ls -lah
      --no-cache            No cache: Send command to build container without cache
      --rebuild-image       Rebuild image: Rebuild image if the action directory or the Dockerfile has changed
      --remote-copy-back    Remote copy back: Copies the working directory back from the container. Works only if the runtime is remote.
      --remote-runtime      Remote runtime: Forces the container runtime to be used as remote. Copies the working directory to a container volume. Local binds are not used.
      --remove-image        Remove Image: Remove an image after execution of action

Global Flags:
      --log-format LogFormat    log format, can be: pretty, plain or json (default pretty)
      --log-level logLevelStr   log level, same as -v, can be: DEBUG, INFO, WARN, ERROR or NONE (default NONE)
  -q, --quiet                   disable output to the console
  -v, --verbose count           log verbosity level, use -vvvv DEBUG, -vvv INFO, -vv WARN, -v ERROR
```

### Mounts/Volumes in container runtime

To follow the context on action execution, 2 mounts are passed to the execution environment:
1. `/host`
2. `/action`

If run in the local runtime (docker):

1. Current working directory is mounted to `/host`, Docker equivalent `$(pwd):/host`
2. Action directory is mounted to `/action`, Docker equivalent `./action/dir:/action`

If run in the remote runtime (docker, kubernetes) or with a flag `--remote-runtime`:

1. Current working directory is copied to a new volume `volume_host:/host`
2. Action directory is copied to a new volume `volume_action:/action`

To copy back the result of the execution, use `--remote-copy-back` flag.

### Environment Variables in Container Runtime

Environment variables are passed to Docker containers through the `runtime.env` configuration in the action YAML file. There are key differences between how container and shell runtimes handle environment variables:

#### Container Runtime Environment Variable Behavior

**Explicit Definition Required**: Unlike shell runtime which inherits all host environment variables, container runtime only passes explicitly defined environment variables from the action YAML.

**Host Environment Variable Access**: To access host environment variables, you must explicitly reference them using `${VAR}` syntax in the action definition:

```yaml
runtime:
  type: container
  image: alpine:latest
  env:
    # Static environment variable
    ACTION_ENV: "static_value"

    # Dynamic environment variable from host
    USER_NAME: ${USER}

    # Environment variable with fallback (if HOST_VAR is not set, uses "default")
    HOST_SETTING: ${HOST_VAR-default}
```

**Environment Variable Expansion**: During action loading, the system expands `${VAR}` patterns using the host's environment variables via `os.Expand()`. This happens before the container is created.

**Supported Formats**: Environment variables can be defined using either YAML map or array syntax:

```yaml
# Map syntax (key-value pairs)
runtime:
  env:
    KEY1: value1
    KEY2: ${HOST_VAR}

# Array syntax (KEY=value strings)
runtime:
  env:
    - KEY1=value1
    - KEY2=${HOST_VAR}
```

#### Shell Runtime vs Container Runtime

**Shell Runtime**:
- Inherits all host environment variables automatically
- Additional variables defined in `runtime.env` are appended
- Uses: `cmd.Env = append(os.Environ(), rt.Shell.Env...)`

**Container Runtime**:
- Only passes explicitly defined environment variables
- Host variables must be referenced with `${VAR}` syntax
- No automatic inheritance of host environment

#### Example

For a practical example, see the [envvars action](../example/actions/envvars/action.yaml) which demonstrates both static and dynamic environment variable usage in containers.
