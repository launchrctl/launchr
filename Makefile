GOPATH?=$(HOME)/go
FIRST_GOPATH:=$(firstword $(subst :, ,$(GOPATH)))

# Build available information.
GIT_HASH:=$(shell git log --format="%h" -n 1 2> /dev/null)
GIT_BRANCH:=$(shell git rev-parse --abbrev-ref HEAD)
APP_VERSION:="$(GIT_BRANCH)-$(GIT_HASH)"
GOPKG:=github.com/launchrctl/launchr

DEBUG?=0
ifeq ($(DEBUG), 1)
    LDFLAGS_EXTRA=
    BUILD_OPTS=-gcflags "all=-N -l"
else
    LDFLAGS_EXTRA=-s -w
    BUILD_OPTS=-trimpath
endif

BUILD_ENVPARMS:=CGO_ENABLED=0

GOBIN:=$(FIRST_GOPATH)/bin
LOCAL_BIN:=$(CURDIR)/bin

# Linter config.
GOLANGCI_BIN:=$(LOCAL_BIN)/golangci-lint
GOLANGCI_TAG:=1.64.5

.PHONY: all
all: deps test build

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
	$(info Building launchr...)
# Application related information available on build time.
	$(eval LDFLAGS:=-X '$(GOPKG).name=launchr' -X '$(GOPKG).version=$(APP_VERSION)' $(LDFLAGS_EXTRA))
	$(eval BIN?=$(LOCAL_BIN)/launchr)
	go generate ./...
	$(BUILD_ENVPARMS) go build -ldflags "$(LDFLAGS)" $(BUILD_OPTS) -o $(BIN) ./cmd/launchr

# Install launchr
.PHONY: install
install: all
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
