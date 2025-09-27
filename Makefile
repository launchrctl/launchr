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
GOLANGCI_TAG:=2.5.0

GOTESTFMT_BIN:=$(GOBIN)/gotestfmt

# Color definitions
RED=\033[0;31m
GREEN=\033[0;32m
YELLOW=\033[0;33m
BLUE=\033[0;34m
MAGENTA=\033[0;35m
CYAN=\033[0;36m
WHITE=\033[0;37m
BOLD=\033[1m
RESET=\033[0m

# Disable colors on Windows.
ifeq ($(OS),Windows_NT)
    RED=
    GREEN=
    YELLOW=
    BLUE=
    MAGENTA=
    CYAN=
    WHITE=
    BOLD=
    RESET=
endif

# Print functions
define print_header
	@echo "$(BOLD)$(CYAN)╔═════════════════════════════════════════════════════════════╗$(RESET)"
	@echo "$(BOLD)$(CYAN)║                           LAUNCHR                           ║$(RESET)"
	@echo "$(BOLD)$(CYAN)╚═════════════════════════════════════════════════════════════╝$(RESET)"
endef

define print_success
	@echo "$(BOLD)$(GREEN)✅ $(1)$(RESET)"
	@echo
endef

define print_info
	@echo "$(BOLD)$(BLUE)📋 $(1)$(RESET)"
	@echo
endef

define print_warning
	@echo "$(BOLD)$(YELLOW)⚠️  $(1)$(RESET)"
	@echo
endef

define print_error
	@echo "$(BOLD)$(RED)❌ $(1)$(RESET)"
	@echo
endef

define print_step
	@echo "$(BOLD)$(MAGENTA)🔧 $(1)$(RESET)"
endef

.PHONY: all
all: banner deps test-short build
	$(call print_success,"🎉 All tasks completed successfully!")

.PHONY: banner
banner:
	$(call print_header)
	@echo "$(BOLD)$(WHITE)📦 Version: $(APP_VERSION)$(RESET)"
	@echo "$(BOLD)$(WHITE)🌿 Branch:  $(GIT_BRANCH)$(RESET)"
	@echo "$(BOLD)$(WHITE)🔗 Hash:    $(GIT_HASH)$(RESET)"
	@echo

# Install go dependencies
.PHONY: deps
deps:
	$(call print_step,"Installing go dependencies...")
	@go mod download
	$(call print_success,"Dependencies installed successfully!")

# Run all tests
.PHONY: test
test: .install-gotestfmt
	$(call print_step,"Running all tests...")
	@go test -json -v ./... | $(GOTESTFMT_BIN) -hide all && \
	echo "$(BOLD)$(GREEN)🧪 ✅ All tests passed$(RESET)" || \
	echo "$(BOLD)$(RED)🧪 ❌ Some tests failed$(RESET)"
	@echo

# Run short tests
.PHONY: test-short
test-short: .install-gotestfmt
	$(call print_step,"Running short tests...")
	@go test -json -short -v ./... | $(GOTESTFMT_BIN) -hide all && \
	echo "$(BOLD)$(GREEN)🧪 ✅ All short tests passed$(RESET)" || \
	echo "$(BOLD)$(RED)🧪 ❌ Some short tests failed$(RESET)"
	@echo

# Build launchr
.PHONY: build
build:
	$(call print_step,"Building launchr...")
# Application related information available on build time.
	$(eval LDFLAGS:=-X '$(GOPKG).name=launchr' -X '$(GOPKG).version=$(APP_VERSION)' $(LDFLAGS_EXTRA))
	$(eval BIN?=$(LOCAL_BIN)/launchr)
	@go generate ./...
	@$(BUILD_ENVPARMS) go build -ldflags "$(LDFLAGS)" $(BUILD_OPTS) -o $(BIN) ./cmd/launchr
	$(call print_success,"🔨 Build completed: $(BIN)")

# Install launchr
.PHONY: install
install: all
	$(call print_step,"Installing launchr to GOPATH...")
	@cp $(LOCAL_BIN)/launchr $(GOBIN)/launchr
	$(call print_success,"🚀 launchr installed to $(GOBIN)/launchr")

# Install and run linters
.PHONY: lint
lint: .install-lint .lint-fix

# Install golangci-lint binary
.PHONY: .install-lint
.install-lint:
ifeq ($(wildcard $(GOLANGCI_BIN)),)
	$(call print_step,"Installing golangci-lint v$(GOLANGCI_TAG)...")
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(LOCAL_BIN) v$(GOLANGCI_TAG)
	$(call print_success,"golangci-lint installed!")
endif

# Install gotestfmt binary
.PHONY: .install-gotestfmt
.install-gotestfmt:
ifeq ($(wildcard $(GOTESTFMT_BIN)),)
	$(call print_step,"Installing gotestfmt...")
	@go install github.com/gotesttools/gotestfmt/v2/cmd/gotestfmt@latest
	$(call print_success,"gotestfmt installed!")
endif

# Runs linters
.PHONY: .lint-fix
.lint-fix:
	$(call print_step,"Running linters with auto-fix...")
	@$(GOLANGCI_BIN) run --fix ./... && \
	echo "$(BOLD)$(GREEN)🔍 ✅ All linting checks passed$(RESET)" || \
	echo "$(BOLD)$(YELLOW)🔍 ⚠️  Some linting issues found - please review$(RESET)"
	@echo

.PHONY: .lint
.lint:
	$(call print_step,"Running linters...")
	@$(GOLANGCI_BIN) run && \
	echo "$(BOLD)$(GREEN)🔍 ✅ All linting checks passed$(RESET)" || \
	echo "$(BOLD)$(YELLOW)🔍 ⚠️ Some linting issues found - please review$(RESET)"
	@echo

# Clean build artifacts
.PHONY: clean
clean:
	$(call print_step,"Cleaning build artifacts...")
	@rm -rf $(LOCAL_BIN)
	$(call print_success,"🧹 Cleanup completed!")

# Show help
.PHONY: help
help:
	$(call print_header)
	@echo "$(BOLD)$(WHITE)Available targets:$(RESET)"
	@echo ""
	@echo "  $(BOLD)$(GREEN)all$(RESET)         🎯 Run deps, test, and build"
	@echo "  $(BOLD)$(GREEN)deps$(RESET)        📦 Install go dependencies"
	@echo "  $(BOLD)$(GREEN)test$(RESET)        🧪 Run all tests"
	@echo "  $(BOLD)$(GREEN)test-short$(RESET)  ⚡ Run short tests only"
	@echo "  $(BOLD)$(GREEN)build$(RESET)       🔨 Build launchr binary"
	@echo "  $(BOLD)$(GREEN)install$(RESET)     🚀 Install launchr to GOPATH"
	@echo "  $(BOLD)$(GREEN)lint$(RESET)        🔍 Run linters with auto-fix"
	@echo "  $(BOLD)$(GREEN)clean$(RESET)       🧹 Clean build artifacts"
	@echo "  $(BOLD)$(GREEN)help$(RESET)        ❓ Show this help message"
	@echo ""
	@echo "$(BOLD)$(CYAN)Environment variables:$(RESET)"
	@echo "  $(BOLD)$(YELLOW)DEBUG=1$(RESET)     Enable debug build"
	@echo "  $(BOLD)$(YELLOW)BIN=path$(RESET)    Custom binary output path"
	@echo ""

# Default target shows help
.DEFAULT_GOAL := help
