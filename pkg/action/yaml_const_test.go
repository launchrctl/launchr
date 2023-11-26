package action

const validEmptyVersionYaml = `
action:
  title: Title
  description: Description
  image: python:3.7-slim
  command: python3 {{ .Arg0 }}
`

const validFullYaml = `
version: "1"
action:
  title: Title
  description: Description
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
    - name: arg2
      title: Argument 2
      description: Argument 2 description
  options:
    - name: opt1
      title: Option 1
      description: Option 1 description
    - name: opt-1
      title: Option 1
      description: Option 1 description
    - name: opt2
      title: Option 2
      description: Option 2 description
      type: boolean
      required: true
    - name: opt3
      title: Option 3
      description: Option 3 description
      type: integer
    - name: opt4
      title: Option 4
      description: Option 4 description
      type: number
    - name: optarr
      title: Option 5
      description: Option 5 description
      type: array
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

const validCmdArrYaml = `
action:
  title: Title
  image: python:3.7-slim
  command:
    - /bin/sh
    - -c
    - for i in $(seq 3); do echo $$i; sleep 1; done
`

const invalidCmdObjYaml = `
action:
  title: Title
  image: python:3.7-slim
  command:
    line1: /bin/sh
    line2: -c
    line3: for i in $(seq 3); do echo $$i; sleep 1; done
`

const invalidCmdArrVarYaml = `
action:
  title: Title
  image: python:3.7-slim
  command:
    - /bin/sh
    - line2: -c
      line3: for i in $(seq 3); do echo $$i; sleep 1; done
`

const unsupportedVersionYaml = `
version: "2"
action:
  title: Title
  image: python:3.7-slim
  command: python3
`

const invalidEmptyImgYaml = `
version:
action:
  title: Title
  command: python3
`

const invalidEmptyStrImgYaml = `
version:
action:
  title: Title
  command: python3
  image: ""
`

const invalidEmptyCmdYaml = `
version: "1"
action:
  title: Title
  image: python:3.7-slim
`

const invalidEmptyArrCmdYaml = `
version: "1"
action:
  title: Title
  image: python:3.7-slim
  command: []
`

// Arguments definition.
const invalidArgsStringYaml = `
version: "1"
action:
  title: Title
  arguments: "invalid"
  image: python:3.7-slim
  command: ls
`

const invalidArgsStringArrYaml = `
version: "1"
action:
  title: Title
  arguments: ["invalid"]
  image: python:3.7-slim
  command: ls
`

const invalidArgsObjYaml = `
version: "1"
action:
  title: Title
  arguments:
    objKey: "invalid"
  image: python:3.7-slim
  command: ls
`

const invalidArgsEmptyNameYaml = `
version: "1"
action:
  title: Title
  arguments:
    - title: arg1
  image: python:3.7-slim
  command: ls
`

const invalidArgsNameYaml = `
version: "1"
action:
  title: Title
  arguments:
    - name: 0arg
  image: python:3.7-slim
  command: ls
`

// Options definition.
const invalidOptsStrYaml = `
version: "1"
action:
  title: Title
  options: "invalid"
  image: python:3.7-slim
  command: ls
`

const invalidOptsStrArrYaml = `
version: "1"
action:
  title: Title
  options: ["invalid"]
  image: python:3.7-slim
  command: ls
`

const invalidOptsObjYaml = `
version: "1"
action:
  title: Verb
  options:
    objKey: "invalid"
  image: python:3.7-slim
  command: ls
`

const invalidOptsEmptyNameYaml = `
version: "1"
action:
  title: Title
  options:
    - title: opt
  image: python:3.7-slim
  command: ls
`

const invalidOptsNameYaml = `
version: "1"
action:
  title: Title
  options:
    - name: opt+name
  image: python:3.7-slim
  command: ls
`

const invalidDupArgsOptsNameYaml = `
version: "1"
action:
  title: Title
  arguments:
    - name: dupName
  options:
    - name: dupName
  image: python:3.7-slim
  command: ls
`

const invalidMultipleErrYaml = `
version: "1"
action:
  title: Title
  arguments:
    - name: dupName
  options:
    - name: dupName
    - title: otherTitle
  image: python:3.7-slim
  command: ls
`

const invalidJSONSchemaTypeYaml = `
version: "1"
action:
  title: Title
  arguments:
    - name: dupName
      type: unsup
  image: python:3.7-slim
  command: ls
`

const invalidJSONSchemaDefaultYaml = `
version: "1"
action:
  title: Title
  options:
    - name: dupName
      type: object
      default:
  image: python:3.7-slim
  command: ls
`

// Build image key.
const validBuildImgShortYaml = `
action:
  image: python:3.7-slim
  build: ./
  command: ls
`

const validBuildImgLongYaml = `
action:
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
  command: ls
`

// Extra hosts key.
const validExtraHostsYaml = `
action:
  image: python:3.7-slim
  extra_hosts:
    - "host.docker.internal:host-gateway"
    - "example.com:127.0.0.1"
  command: ls
`

const invalidExtraHostsYaml = `
action:
  image: python:3.7-slim
  extra_hosts: "host.docker.internal:host-gateway"
  command: ls
`

// Environmental variables.
const validEnvArr = `
action:
  image: my/image:v1
  command: ls
  env:
    - MY_ENV_1=test1
    - MY_ENV_2=test2
`

const validEnvObj = `
action:
  image: my/image:v1
  command: ls
  env:
    MY_ENV_1: test1
    MY_ENV_2: test2
`

const invalidEnv = `
action:
  image: my/image:v1
  command: ls
  env:
    - MY_ENV_1=test1
    MY_ENV_2: test2
`

const invalidEnvStr = `
action:
  image: my/image:v1
  command: ls
  env: MY_ENV_1=test1
`

const invalidEnvObj = `
action:
  image: my/image:v1
  command: ls
  env:
    MY_ENV_1: { MY_ENV_2: test2 }
`

// Unescaped template strings.
const validUnescTplStr = `
action:
  image: {{ .A1 }}
  command:    {{ .A1 }}
  env:
    - {{ .A2 }} {{ .A3 }}
    - {{ .A2 }} {{ .A3 }} asafs
`

const invalidUnescUnsupKeyTplStr = `
action:
  image: {{ .A1 }}:latest
  {{ .A1 }}: ls
`

const invalidUnescUnsupArrTplStr = `
action:
  image: {{ .A1 }}
  command: [{{ .A1 }}, {{ .A1 }}]
`
