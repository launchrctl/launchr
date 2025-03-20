# Launchr

Launchr is a versatile CLI action runner that executes tasks defined in local or embeded yaml files across multiple runtimes:
- Short-lived container (docker)
- Shell (host)
- Golang (as plugin)

It supports:
- Arguments and options
- Automatic action discovery
- Automatic path-based naming of local actions
- Seamless extensibility via a plugin system

Actions are defined in `action.yaml` files:
- either on local filesystem: Useful for project-specific actions
- or embeded as plugin: Useful for common and shared actions

You can find action examples [here](example), here and in the [documentation](docs).

Launchr has a plugin system that allows to extend its functionality. See [core plugins](plugins), [official plugins](https://github.com/launchrctl#org-repositories) and [documentation](docs).


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
make
bin/launchr --version
```

## Development

The `launchr`  can be built with a `make` to `bin` directory:
```shell
make
```
It is also supported to make a build to use with `dlv` for debug:
```shell
make DEBUG=1
```

Useful make commands:
1. Fetch dependencies - `make deps`
2. Test the code - `make test`
3. Lint the code - `make lint`


## Publishing a new release

- Create a new Github release [from UI](https://github.com/launchrctl/launchr/releases/new)
- Github Action will compile new binaries using [goreleaser](https://goreleaser.com/) and attach them to release

