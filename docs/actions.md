# Actions

Actions give an ability to run arbitrary commands in containers.

## Supported container engines
1. `docker`, see [installation guide](https://docs.docker.com/engine/install/)
2. TBD

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
  image: python:3.7-slim
  command: 
    - python3 
    - {{ .myArg1 }} {{ .myArg2 }}
    - {{ .optStr }}
    - ${ENV_VAR}
```

See more examples of action definition in [actions.schema.md](actions.schema.md)

## Actions discovery

The action files must preserve a tree structure like `**/**/actions/*/action.yaml` to be discovered.  
Example:
```
foundation
└── software
    └── flatcar
        └── actions
            └── bump
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
...
Discovered actions:
  foundation.software.flatcar:bump  Verb: Handles some logic
  integration.application.bus:watch Verb: Handles some logic
  platform:build                    Verb: Handles some logic
  platform:bump                     Verb: Handles some logic
...
```

### Action execution

To run the command simply run:
```shell
$ launchr platform:build ...
```

To get more help for the action:
```shell
$ launchr platform:build --help
Verb: Handles some logic

Usage:
  launchr platform:build args[1] args[2] [flags]

Flags:
  -h, --help             help for platform:build
      --opt1 string      Option 1: Some additional info for option
      --opt2             Option 2: Some additional info for option
      --opt3 int         Option 3: Some additional info for option
      --opt4 float       Option 4: Some additional info for option
      --optarr strings   Option 4: Some additional info for option

Global Flags:
  -q, --quiet           log only fatal errors
  -v, --verbose count   log verbosity level, use -vvv DEBUG, -vv WARN, -v INFO
```

### Container environment flags

 * `--entrypoint`      Entrypoint: Overwrite the default ENTRYPOINT of the image
 * `--exec`            Exec: Overwrite CMD definition of the container
 * `--no-cache`        No cache: Send command to build container without cache
 * `--remove-image`    Remove Image: Remove an image after execution of action
 * `--use-volume-wd`   Use volume as a WD: Copy the working directory to a container volume and not bind local paths. Usually used with remote environments.


### Mounts in execution environment

To follow the context on action execution, 2 mounts are passed to the execution environment:
1. `/host` - current working directory
2. `/action` - action directory
