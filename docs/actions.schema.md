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

For templating, standard Go templating engine is used. 
Refer to [documentation](https://pkg.go.dev/text/template).   

Arguments and Options are available by their machine names - `{{ .myArg1 }}`, `{{ .optStr }}`, `{{ .optArr }}`, etc.

Environment variables:

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

## Build image

Images may be built in place. `build` directive describes the working directory on build.  
Image name is used to tag the built image.

Short declaration:
```yaml
...
  image: my/image:version
  build: ./ # Build working directory
...
```

Long declaration:
```yaml
...
  image: my/image:version
  build:
    context: ./
    buildfile: test.Dockerfile
    args:
      arg1: val1
      arg2: val2
    tags:
      - alt/tag:version
      - alt/tag:version2
...
```

1. `context` - build working directory
2. `buildfile` - build file relative to context directory, can't be outside of the `context` directory.
3. `args` - arguments passed to the `buildfile`
4. `tags` - tags for a build image

## Extra hosts

Extra hosts may be passed to be resolved inside the action environment:
```yaml
...
  extra_hosts:
    - "host.docker.internal:host-gateway"
    - "example.com:127.0.0.1"
...
```

## Environment variables

To pass environment variables to the execution environment:
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
