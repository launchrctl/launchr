package action

var validEmptyVersionYaml = `
action:
  title: Title
  description: Description
  image: python:3.7-slim
  command: python3 {{ .Arg0 }}
`

var validFullYaml = `
version: "1"
action:
  title: Title
  description: Description
  arguments:
    - name: arg1
      title: Argument 1
      description: Argument 1 description
    - name: arg2
      title: Argument 2
      description: Argument 2 description
  options:
    - name: opt1
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
    - "{{ .arg2 }} {{ .arg1 }}"
    - "{{ .opt3 }} {{ .opt2 }} {{ .opt1 }} {{ .opt4 }} {{ .optarr }}"
    - ${TEST_ENV_1} ${TEST_ENV_UND}
    - "${TEST_ENV_1} ${TEST_ENV_UND}"
`

// @todo invalid variables to replace

var validCmdArrYaml = `
action:
  title: Title
  image: python:3.7-slim
  command:
    - /bin/sh
    - -c
    - for i in $(seq 3); do echo $$i; sleep 1; done
`

var invalidCmdObjYaml = `
action:
  title: Title
  image: python:3.7-slim
  command:
    line1: /bin/sh
    line2: -c
    line3: for i in $(seq 3); do echo $$i; sleep 1; done
`

var invalidCmdArrVarYaml = `
action:
  title: Title
  image: python:3.7-slim
  command:
    - /bin/sh
    - line2: -c
      line3: for i in $(seq 3); do echo $$i; sleep 1; done
`

var unsupportedVersionYaml = `
version: "2"
action:
  title: Title
  image: python:3.7-slim
  command: python3
`

var invalidEmptyImgYaml = `
version: "1"
action:
  title: Title
  command: python3
`

var invalidEmptyCmdYaml = `
version: "1"
action:
  title: Title
  image: python:3.7-slim
`

var invalidArgsStringYaml = `
version: "1"
action:
  title: Title
  arguments: "invalid"
  image: python:3.7-slim
  command: ls
`

var invalidArgsStringArrYaml = `
version: "1"
action:
  title: Title
  arguments: ["invalid"]
  image: python:3.7-slim
  command: ls
`

var invalidArgsObjYaml = `
version: "1"
action:
  title: Title
  arguments:
    objKey: "invalid"
  image: python:3.7-slim
  command: ls
`

var invalidArgsEmptyNameYaml = `
version: "1"
action:
  title: Title
  arguments:
    - title: arg1
  image: python:3.7-slim
  command: ls
`

var invalidArgsNameYaml = `
version: "1"
action:
  title: Title
  arguments:
    - name: 0arg
  image: python:3.7-slim
  command: ls
`

var invalidOptsStrYaml = `
version: "1"
action:
  title: Title
  options: "invalid"
  image: python:3.7-slim
  command: ls
`

var invalidOptsStrArrYaml = `
version: "1"
action:
  title: Title
  options: ["invalid"]
  image: python:3.7-slim
  command: ls
`

var invalidOptsObjYaml = `
version: "1"
action:
  title: Verb
  options:
    objKey: "invalid"
  image: python:3.7-slim
  command: ls
`

var invalidOptsEmptyNameYaml = `
version: "1"
action:
  title: Title
  options:
    - title: opt
  image: python:3.7-slim
  command: ls
`

var invalidOptsNameYaml = `
version: "1"
action:
  title: Title
  options:
    - name: opt-name
  image: python:3.7-slim
  command: ls
`

var invalidDupArgsOptsNameYaml = `
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

var validBuildImgShortYaml = `
action:
  image: python:3.7-slim
  build: ./
  command: ls
`

var validBuildImgLongYaml = `
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

var validExtraHostsYaml = `
action:
  image: python:3.7-slim
  extra_hosts:
    - "host.docker.internal:host-gateway"
    - "example.com:127.0.0.1"
  command: ls
`

var validEnvArr = `
action:
  image: my/image:v1
  command: ls
  env:
    - MY_ENV_1=test1
    - MY_ENV_2=test2
`

var validEnvObj = `
action:
  image: my/image:v1
  command: ls
  env:
    MY_ENV_1: test1
    MY_ENV_2: test2
`

var invalidEnv = `
action:
  image: my/image:v1
  command: ls
  env:
    - MY_ENV_1=test1
    MY_ENV_2: test2
`

var validUnescTplStr = `
action:
  image: {{ .A1 }}
  command:    {{ .A1 }}
  env:
    - {{ .A2 }} {{ .A3 }}
    - {{ .A2 }} {{ .A3 }} asafs
`

var invalidUnescUnsupKeyTplStr = `
action:
  image: {{ .A1 }}:latest
  {{ .A1 }}: ls
`

var invalidUnescUnsupArrTplStr = `
action:
  image: {{ .A1 }}
  command: [{{ .A1 }}, {{ .A1 }}]
`
