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
└── upgrade
    └── action.yaml
integration
└── application
    └── bus
        └── actions
            └── watch
                └── action.yaml
platform
└── actions
    ├── build
    │   ├── build.sh
    │   └── action.yaml
    └── bump
        ├── bump.py
        └── action.yaml
```

The example structure produces the following commands:

```shell
$ launchr --help
launchr is a versatile CLI action runner that executes tasks defined in local or embeded yaml files across multiple runtimes
...
Actions:
  upgrade                           Upgrade: description functionality
  foundation.software.flatcar:bump  Bump: foundation.software.flatcar:bump description
  integration.application.bus:watch Watch: integration.application.bus:watch description
  platform:build                    Platform Build: platform:build description
  platform:bump                     Platform Bump: platform:bump description
...
```

### Action execution

To run the command simply run:
```shell
$ launchr platform:build [args...] [--options...]
```

To get more help for the action:


```shell
$ launchr platform:build --help

Platform build: platform:build description

Usage:
  launchr example.actions.platform:build argStr [argInt] [flags]

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