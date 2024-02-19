# Actions YAML features

## Action declaration

Basic action definition must have `image` and `command` to run the command in the environment.

```yaml
action:
  title: Action name
  description: Long description
  image: alpine:latest
  command:
    - ls
    - -lah
```

## Arguments and options

Arguments and options are defined in `action.yaml`, parsed according to the schema and replaced on run.

### Arguments

```yaml
...
  arguments:
    - name: myArg1
      title: Argument 1
      description: Some additional info for arg
    - name: MyArg2
      title: Argument 2
      description: Some additional info for arg
...
```

### Options

```yaml
...
  options:
    - name: optStr
      title: Option string
      description: Some additional info for option
    - name: optBool
      title: Option bool
      description: Some additional info for option
      type: boolean
    - name: optInt
      title: Option int
      description: Some additional info for option
      type: integer
    - name: optNum
      title: Option float
      description: Some additional info for option
      type: number
    - name: optArr
      title: Option array
      description: Some additional info for option
      type: array
...
```

### Variable types

Arguments and options values declaration follows [JSON Schema](https://json-schema.org/) (not yet actually).

**Supported types:**
1. `string`
2. `boolean`
3. `integer`
4. `number` - float64 values
5. `array` (currently array of strings only)

Arguments can only be of type `string` and are always required.

## Templating of action file

The action provides basic templating for all file based on arguments, options and environment variables.

### Arguments and options

For templating, standard Go templating engine is used.
Refer to [documentation](https://pkg.go.dev/text/template).

Arguments and Options are available by their machine names - `{{ .myArg1 }}`, `{{ .optStr }}`, `{{ .optArr }}`, etc.

### Predefined variables:

1. `current_uid` - current user ID. In Windows environment set to 0.
2. `current_gid` - current group ID. In Windows environment set to 0.
3. `current_working_dir` - app working directory.
4. `actions_base_dir` - actions base directory where the action was found. By default, current working directory,
    but other paths may be provided by plugins.
5. `action_dir` - directory of the action file.

### Environment variables:

| __Expression__     | __Meaning__                                                          |
|--------------------|----------------------------------------------------------------------|
| `${var}`           | Value of var (same as `$var`)                                        |
| `${var-$DEFAULT}`  | If var not set, evaluate expression as $DEFAULT                      |
| `${var:-$DEFAULT}` | If var not set or is empty, evaluate expression as $DEFAULT          |
| `${var=$DEFAULT}`  | If var not set, evaluate expression as $DEFAULT                      |
| `${var:=$DEFAULT}` | If var not set or is empty, evaluate expression as $DEFAULT          |
| `${var+$OTHER}`    | If var set, evaluate expression as $OTHER, otherwise as empty string |
| `${var:+$OTHER}`   | If var set, evaluate expression as $OTHER, otherwise as empty string |
| `$$var`            | Escape expressions. Result will be `$var`.                           |

### Example

```yaml
...
  arguments:
    - name: myArg1
    - name: MyArg2
...
  options:
    - name: optStr
    - name: optBool
...
  image: {{ .optStr }}:latest
  command:
    - {{ .myArg1 }} {{ .MyArg2 }}
    - {{ .optBool }}
```

## Command

Command can be written in 2 forms - as a string and as an array:
```yaml
...
  command: ls
...
```

```yaml
...
  command: ["ls", "-al"]
...
```

```yaml
...
  command:
    - ls
    - -al
...
```

It is recommended to use array form for multiple arguments.
## Environment variables

To pass environment variables to the execution environment, add `env` section (outside of `build section`):
```yaml
  env:
    - ENV1=val1
    - ENV2=$ENV2
    - ENV3=${ENV3}
```
Or in map style:
```yaml
  env:
    ENV1: val1
    ENV2: $ENV2
    ENV3: ${ENV3}
```
For instance:
```yaml
action:
  title: Test
  description: Test
  image: test:latest
  env:
    ACTION_ENV: some_value
  build:
    context: ./
    args:
      USER_ID: {{ .current_uid }}
      GROUP_ID: {{ .current_gid }}
  command:
    - sh
    - /action/main.sh
```
Renders as:
```
+ echo 'ACTION_ENV=some_value'
ACTION_ENV=some_value
```
Or
```yaml
action:
  title: Test
  description: Test
  image: test:latest
  env:
    ACTION_ENV: ${HOST_ENV}
  build:
    context: ./
    args:
      USER_ID: {{ .current_uid }}
      GROUP_ID: {{ .current_gid }}
  command:
    - sh
    - /action/main.sh
```
Renders as:
```
+ echo 'ACTION_ENV=var_value_from_host'
ACTION_ENV=var_value_from_host
```

## Extra hosts

Extra hosts may be passed to be resolved inside the action environment:
```yaml
  extra_hosts:
    - "host.docker.internal:host-gateway"
    - "example.com:127.0.0.1"
```
Renders `/etc/hosts` as:
```
+ cat /etc/hosts
...
172.17.0.1	host.docker.internal
127.0.0.1	example.com
```

## Build image

Images may be built in place. `build` directive describes the working directory on build.
Image name is used to tag the built image.

Short declaration:
```yaml
  image: my/image:version
  build: ./ # Build working directory
...
```

Long declaration:
```yaml
  image: my/image:version
  build:
    context: ./
    buildfile: test.Dockerfile
    args:
      arg1: val1
      arg2: val2
...
```

1. `context` - build working directory
2. `buildfile` - build file relative to context directory, can't be outside of the `context` directory.
3. `tags` - tags for a build image
4. `args` - arguments passed to the `buildfile` can be used in Dockerfile, such as:
```yaml
  build:
    context: ./
    args:
      USER_ID: {{ .current_uid }}
      GROUP_ID: {{ .current_gid }}
      USER_NAME: plasma
```
Can be used as:
```
FROM alpine:latest
ARG USER_ID
ARG USER_NAME
ARG GROUP_ID
RUN adduser -D -u ${USER_ID} -g ${GROUP_ID} ${USER_NAME} || true
USER $USER_NAME
```
And renders as:
```
+ whoami
plasma
+ id
uid=1000(plasma) gid=1000(plasma) groups=1000(plasma)
```
