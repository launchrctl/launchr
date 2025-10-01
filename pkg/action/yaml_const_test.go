package action

const validEmptyVersionYaml = `
runtime: plugin
action:
  title: Title
  description: Description
`

const validFullYaml = `
version: "1"
working_directory: '{{ .current_working_dir }}'
action:
  title: Title
  description: Description
  alias:
    - alias1
    - alias2
  arguments:
    - name: arg1
      title: Argument 1
      description: Argument 1 description
    - name: arg-1
      title: Argument 1
      description: Argument 1 description
    - name: arg_12
      title: Argument 1
      description: Argument 1 description
      enum: [arg_12_enum1, arg_12_enum2]
    - name: arg2
      title: Argument 2
      description: Argument 2 description
  options:
    - name: opt1
      title: Option 1 String
      description: Option 1 description
    - name: opt-1
      title: Option 1 String with dash
      description: Option 1 description
    - name: opt2
      title: Option 2 Boolean
      description: Option 2 description
      type: boolean
      required: true
    - name: opt3
      title: Option 3 Integer
      description: Option 3 description
      type: integer
    - name: opt4
      title: Option 4 Number
      description: Option 4 description
      type: number
    - name: optarr
      title: Option 5 Array
      description: Option 5 description
      type: array
runtime:
  type: container
  image: my/image:v1
  build:
    context: ./
    buildfile: test.Dockerfile
    args:
      arg1: val1
      arg2: val2
    tags:
      - my/image:v2
      - my/image:v3
  extra_hosts:
    - "host.docker.internal:host-gateway"
    - "example.com:127.0.0.1"
  env:
    - MY_ENV_1=test1
    - MY_ENV_2=test2
  command:
    - /bin/sh
    - -c
    - ls -lah
    - "{{ .arg2 }} {{ .arg1 }} {{ .arg_1 }} {{ .arg_12 }}"
    - "{{ .opt3 }} {{ .opt2 }} {{ .opt1 }} {{ .opt_1 }} {{ .opt4 }} {{ .optarr }}"
    - ${TEST_ENV_1} ${TEST_ENV_UND}
    - "${TEST_ENV_1} ${TEST_ENV_UND}"
`

const validMultilineYaml = `
action:
  title: Title
runtime:
  type: container
  image: python:3.7-slim
  env:
    MY_MULTILINE_ENV1: "${TEST_MULTILINE_ENV1}"
    MY_MULTILINE_ENV2: ${TEST_MULTILINE_ENV1}
    MY_MULTILINE_ENV3: |
      ${TEST_MULTILINE_ENV1}
  command: [pwd]
`

const validCmdArrYaml = `
action:
  title: Title
runtime:
  type: container
  image: python:3.7-slim
  command:
    - /bin/sh
    - -c
    - for i in $(seq 3); do echo $$i; sleep 1; done
`

const invalidCmdStringYaml = `
action:
  title: Title
runtime:
  type: container
  image: python:3.7-slim
  command: pwd
`

const invalidCmdObjYaml = `
action:
  title: Title
runtime:
  type: container
  image: python:3.7-slim
  command:
    line1: /bin/sh
    line2: -c
    line3: for i in $(seq 3); do echo $$i; sleep 1; done
`

const invalidCmdArrVarYaml = `
action:
  title: Title
runtime:
  type: container
  image: python:3.7-slim
  command:
    - /bin/sh
    - line2: -c
      line3: for i in $(seq 3); do echo $$i; sleep 1; done
`

const unsupportedVersionYaml = `
version: "2"
runtime: plugin
action:
  title: Title
`

const invalidEmptyImgYaml = `
version:
action:
  title: Title
  command: [pwd]
runtime:
  type: container
`

const invalidEmptyStrImgYaml = `
version:
action:
  title: Title
runtime:
  type: container
  command: [pwd]
  image: ""
`

const invalidEmptyCmdYaml = `
version: "1"
action:
  title: Title
runtime:
  type: container
  image: python:3.7-slim
`

const invalidEmptyArrCmdYaml = `
version: "1"
action:
  title: Title
runtime:
  type: container
  image: python:3.7-slim
  command: []
`

// Arguments definition.
const invalidArgsStringYaml = `
version: "1"
runtime: plugin
action:
  title: Title
  arguments: "invalid"
`

const invalidArgsStringArrYaml = `
version: "1"
runtime: plugin
action:
  title: Title
  arguments: ["invalid"]
`

const invalidArgsObjYaml = `
version: "1"
runtime: plugin
action:
  title: Title
  arguments:
    objKey: "invalid"
`

const invalidArgsEmptyNameYaml = `
version: "1"
runtime: plugin
action:
  title: Title
  arguments:
    - title: arg1
`

const invalidArgsNameYaml = `
version: "1"
runtime: plugin
action:
  title: Title
  arguments:
    - name: 0arg
`

const invalidArgsDefaultMismatch = `
version: "1"
runtime: plugin
action:
  title: Title
  arguments:
    - name: arg
      default: 1
`

// Options definition.
const invalidOptsStrYaml = `
version: "1"
runtime: plugin
action:
  title: Title
  options: "invalid"
`

const invalidOptsStrArrYaml = `
version: "1"
runtime: plugin
action:
  title: Title
  options: ["invalid"]
`

const invalidOptsObjYaml = `
version: "1"
runtime: plugin
action:
  title: Verb
  options:
    objKey: "invalid"
`

const invalidOptsEmptyNameYaml = `
version: "1"
runtime: plugin
action:
  title: Title
  options:
    - title: opt
`

const invalidOptsNameYaml = `
version: "1"
runtime: plugin
action:
  title: Title
  options:
    - name: opt+name
`

const invalidDupArgsOptsNameYaml = `
version: "1"
runtime: plugin
action:
  title: Title
  arguments:
    - name: dupName
  options:
    - name: dupName
`

const invalidMultipleErrYaml = `
version: "1"
runtime: plugin
action:
  title: Title
  arguments:
    - name: dupName
  options:
    - name: dupName
    - title: otherTitle
`

const invalidJSONSchemaTypeYaml = `
version: "1"
runtime: plugin
action:
  title: Title
  arguments:
    - name: dupName
      type: unsup
`

// Build image key.
const validBuildImgShortYaml = `
action:
  title: Title
runtime:
  type: container
  image: python:3.7-slim
  build: ./
  command: [pwd]
`

const validBuildImgLongYaml = `
action:
  title: Title
runtime:
  type: container
  image: python:3.7-slim
  build:
    context: ./
    buildfile: Dockerfile
    args:
      arg1: val1
      arg2: val2
    tags:
      - my/image:v1
      - my/image:v2
  command: [pwd]
`

// Extra hosts key.
const validExtraHostsYaml = `
action:
  title: Title
runtime:
  type: container
  image: python:3.7-slim
  extra_hosts:
    - "host.docker.internal:host-gateway"
    - "example.com:127.0.0.1"
  command: [pwd]
`

const invalidExtraHostsYaml = `
action:
  title: Title
runtime:
  type: container
  image: python:3.7-slim
  extra_hosts: "host.docker.internal:host-gateway"
  command: [pwd]
`

// Environmental variables.
const validEnvArr = `
action:
  title: Title
runtime:
  type: container
  image: my/image:v1
  command: [pwd]
  env:
    - MY_ENV_1=test1
    - MY_ENV_2=test2
`

const validEnvObj = `
action:
  title: Title
runtime:
  type: container
  image: my/image:v1
  command: [pwd]
  env:
    MY_ENV_1: test1
    MY_ENV_2: test2
`

const invalidEnv = `
action:
  title: Title
runtime:
  type: container
  image: my/image:v1
  command: [pwd]
  env:
    - MY_ENV_1=test1
    MY_ENV_2: test2
`

const invalidEnvStr = `
action:
  title: Title
runtime:
  type: container
  image: my/image:v1
  command: [pwd]
  env: MY_ENV_1=test1
`

const invalidEnvObj = `
action:
  title: Title
runtime:
  type: container
  image: my/image:v1
  command: [pwd]
  env:
    MY_ENV_1: { MY_ENV_2: test2 }
`

// Unescaped template strings.
const invalidUnescTplStr = `
action:
  title: Title
runtime:
  type: container
  image:   {{ .A1 }}
  command: [pwd]
  env:
    - {{ .A2 }} {{ .A3 }}
    - {{ .A2 }} {{ .A3 }} asafs
`

const validArgString = `
runtime: plugin
action:
  title: Title
  arguments:
    - name: arg_string
      required: true
`

const validArgStringOptional = `
runtime: plugin
action:
  title: Title
  arguments:
    - name: arg_string
      required: false
`

const validArgStringEnum = `
runtime: plugin
action:
  title: Title
  arguments:
    - name: arg_enum
      enum: [enum1, enum2]
      required: true
`

const validArgBoolean = `
runtime: plugin
action:
  title: Title
  arguments:
    - name: arg_boolean
      type: boolean
      required: true
`

const validArgDefault = `
runtime: plugin
action:
  title: Title
  arguments:
    - name: arg_default
      type: string
      default: "default_string"
      required: true
`

const validOptBoolean = `
runtime: plugin
action:
  title: Title
  options:
    - name: opt_boolean
      type: boolean
      required: true
`

const validOptArrayImplicitString = `
runtime: plugin
action:
  title: Title
  options:
    - name: opt_array_str
      type: array
      required: true
`

const validOptArrayStringEnum = `
runtime: plugin
action:
  title: Title
  options:
    - name: opt_array_enum
      type: array
      items:
        type: string
        enum: [enum_arr1, enum_arr2]
      required: true
`

const validOptArrayInt = `
runtime: plugin
action:
  title: Title
  options:
    - name: opt_array_int
      type: array
      items:
        type: integer
      required: true
`

const validOptArrayIntDefault = `
runtime: plugin
action:
  title: Title
  options:
    - name: opt_array_int
      type: array
      items:
        type: integer
      default: [1, 2, 3]
`

const validMultipleArgsAndOpts = `
runtime: plugin
action:
  title: Title
  arguments:
    - name: arg_int
      type: integer
      required: true
    - name: arg_str
      type: string
      required: true
    - name: arg_str2
      type: string
      required: true
    - name: arg_bool
      type: boolean
      required: true
    - name: arg_default
      default: "my_default_string"
  options:
    - name: opt_str
      type: string
    - name: opt_int
      type: integer
      default: 42
    - name: opt_str_default
      type: string
      default: "optdefault"
    - name: opt_str_required
      type: string
      required: true
`

const validPatternFormat = `
runtime: plugin
action:
  title: Title
  arguments:
    - name: arg_email
      type: string
      required: true
      format: email
    - name: arg_pattern
      type: string
      required: true
      pattern: "^[A-Z]+$"
`
