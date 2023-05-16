# Launchr

`launchr`

## Table of contents

* [Usage](#usage)
* [Installation](#installation)
  * [Installation from source](#installation-from-source)
* [Development](#development)

## Usage

Build `launchr` from source locally. Build dependencies:
1. `go >=1.20`, see [installation guide](https://go.dev/doc/install)
2. `make`

Build the `launchr` tool:
```shell
make
bin/launchr --help
```

The documentation for `launchr` usage can be found in [docs](docs).

If you face any issues with `launchr`:
1. Open an issue in the repo. 
2. Share the app version with `launchr --version`

## Installation

### Installation from source

Build dependencies:
1. `go >=1.20`, see [installation guide](https://go.dev/doc/install)
2. `make`

**Global installation**

Install `launchr` globally:
```shell
make install
launchr --version
```

The tool will be installed in `$GOPATH/bin` which is usually `~/go/bin`.  
If `GOPATH` env variable is not available, make sure you have it in `~/.bashrc` or `~/.zhrc`:

```shell
export GOPATH=`go env GOPATH`
export PATH=$PATH:$GOPATH/bin
```

**Local installation**

The tool can be built and run locally:
```shell
make launchr
bin/launchr --version
```

## Development

The `launchr`  can be built with a `make` to `bin` directory:
```shell
make launchr
```
It is also supported to make a build to use with `dlv` for debug:
```shell
make launchr DEBUG=1
```

Useful make commands:
1. Fetch dependencies - `make deps`
2. Test the code - `make test`
3. Lint the code - `make lint`
