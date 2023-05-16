export GOSUMDB=off

GOPATH?=$(HOME)/go
FIRST_GOPATH:=$(firstword $(subst :, ,$(GOPATH)))

# Build available information.
GIT_TAG:=$(shell git describe --exact-match --abbrev=0 --tags 2> /dev/null)
GIT_HASH:=$(shell git log --format="%h" -n 1 2> /dev/null)
GIT_BRANCH:=$(shell git branch 2> /dev/null | grep '*' | cut -f2 -d' ')
GO_VERSION:=$(shell go version)
BUILD_TS:=$(shell date +%FT%T%z)

# App version is sanitized CI branch name, if available.
# Otherwise git branch or commit hash is used.
APP_VERSION:=$(if $(CI_COMMIT_REF_SLUG),$(CI_COMMIT_REF_SLUG),$(if $(GIT_BRANCH),$(GIT_BRANCH),$(GIT_HASH)))

DEBUG?=0
ifeq ($(DEBUG), 1)
    LDFLAGS_EXTRA=
    BUILD_OPTS=-gcflags "all=-N -l"
else
    LDFLAGS_EXTRA=-w -s
    BUILD_OPTS=-trimpath
endif

BUILD_ENVPARMS:=CGO_ENABLED=0

GOBIN:=$(FIRST_GOPATH)/bin
LOCAL_BIN:=$(CURDIR)/bin

# Linter config.
GOLANGCI_BIN:=$(LOCAL_BIN)/golangci-lint
GOLANGCI_TAG:=1.52.2

.PHONY: all
all: launchr

.PHONY: launchr
launchr: APP_NAME=launchr
launchr: SRCPKG=./
launchr: deps test build

# Install go dependencies
.PHONY: deps
deps:
	$(info Installing go dependencies...)
	go mod download

# Run all tests
.PHONY: test
test:
	$(info Running tests...)
	go test ./...

# Build launchr
.PHONY: build
build:
	$(info Building $(APP_NAME)...)
	@if [ -f $(SRCPKG)/gen.go ]; then\
        go run $(SRCPKG)/gen.go $(SRCPKG);\
    fi
# Application related information available on build time.
	$(eval LDFLAGS:=-X 'main.Name=$(APP_NAME)'\
         -X 'main.Version=$(APP_VERSION)'\
         -X 'main.GoVersion=$(GO_VERSION)'\
         -X 'main.BuildDate=$(BUILD_TS)'\
         -X 'main.GitHash=$(GIT_HASH)'\
         -X 'main.GitBranch=$(GIT_BRANCH)' $(LDFLAGS_EXTRA)\
    )
	$(eval BIN?=$(LOCAL_BIN)/$(APP_NAME))
	$(BUILD_ENVPARMS) go build -ldflags "$(LDFLAGS)" $(BUILD_OPTS) -o $(BIN) $(SRCPKG)

# Install launchr
.PHONY: install
install: launchr
install:
	$(info Installing launchr to GOPATH...)
	cp $(LOCAL_BIN)/launchr $(GOBIN)/launchr

# Install and run linters
.PHONY: lint
lint: .install-lint .lint

# Install golangci-lint binary
.PHONY: .install-lint
.install-lint:
ifeq ($(wildcard $(GOLANGCI_BIN)),)
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(LOCAL_BIN) v$(GOLANGCI_TAG)
endif

# Runs linters
.PHONY: .lint
.lint:
	$(info Running lint...)
	$(GOLANGCI_BIN) run --fix ./...
