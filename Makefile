# LDAP Manager - Development Makefile
# Provides comprehensive development, testing, and quality assurance tools

# Version and build information
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT_HASH ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
PACKAGE := github.com/netresearch/ldap-manager/internal

# Go build settings  
GO_VERSION := $(shell go version | awk '{print $$3}')
LDFLAGS := -s -w -X '$(PACKAGE).Version=$(VERSION)' -X '$(PACKAGE).CommitHash=$(COMMIT_HASH)' -X '$(PACKAGE).BuildTimestamp=$(BUILD_TIME)'
BUILDFLAGS := -ldflags="$(LDFLAGS)" -trimpath

# Docker settings
DOCKER_IMAGE := ldap-manager
DOCKER_TAG := latest

# Colors for output
RED := \033[0;31m
GREEN := \033[0;32m
YELLOW := \033[0;33m
BLUE := \033[0;34m
RESET := \033[0m

.PHONY: help setup build test lint clean dev docker

# Default target
all: setup lint test build

## Help: Show this help message
help:
	@echo "$(BLUE)LDAP Manager - Development Tools$(RESET)"
	@echo ""
	@echo "$(GREEN)Available targets:$(RESET)"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  $(YELLOW)%-20s$(RESET) %s\n", $$1, $$2}' $(MAKEFILE_LIST)
	@echo ""
	@echo "$(GREEN)Build Information:$(RESET)"
	@echo "  Version:     $(VERSION)"
	@echo "  Commit:      $(COMMIT_HASH)" 
	@echo "  Build Time:  $(BUILD_TIME)"
	@echo "  Go Version:  $(GO_VERSION)"

## Setup: Install all dependencies and tools
setup: setup-go setup-node setup-tools
	@echo "$(GREEN)✓ Setup complete$(RESET)"

## Setup Go: Download Go dependencies
setup-go:
	@echo "$(BLUE)Installing Go dependencies...$(RESET)"
	@go mod download
	@go mod tidy

## Setup Node: Install Node.js dependencies  
setup-node:
	@echo "$(BLUE)Installing Node.js dependencies...$(RESET)"
	@pnpm install

## Setup Tools: Install development tools
setup-tools:
	@echo "$(BLUE)Installing Go development tools...$(RESET)"
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install honnef.co/go/tools/cmd/staticcheck@latest
	@go install github.com/securecodewarrior/govulncheck@latest || go install golang.org/x/vuln/cmd/govulncheck@latest
	@go install github.com/fzipp/gocyclo/cmd/gocyclo@latest
	@go install golang.org/x/tools/cmd/goimports@latest
	@go install mvdan.cc/gofumpt@latest
	@go install github.com/kisielk/errcheck@latest
	@go install github.com/gordonklaus/ineffassign@latest
	@go install github.com/mdempsky/maligned@latest
	@go install github.com/a-h/templ/cmd/templ@latest

## Setup Hooks: Install pre-commit hooks
setup-hooks:
	@echo "$(BLUE)Installing pre-commit hooks...$(RESET)"
	@pip install pre-commit || echo "⚠️ pre-commit not installed, install with: pip install pre-commit"
	@pre-commit install
	@echo "$(GREEN)✓ Pre-commit hooks installed$(RESET)"

## Build: Build the application binary
build: build-assets
	@echo "$(BLUE)Building application...$(RESET)"
	@CGO_ENABLED=0 go build $(BUILDFLAGS) -o bin/ldap-manager .
	@echo "$(GREEN)✓ Build complete: bin/ldap-manager$(RESET)"

## Build Assets: Build CSS and template assets
build-assets:
	@echo "$(BLUE)Building assets...$(RESET)"
	@pnpm build:assets

## Build Release: Build optimized release binary
build-release: build-assets
	@echo "$(BLUE)Building release binary...$(RESET)"
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(BUILDFLAGS) -o bin/ldap-manager-linux-amd64 .
	@CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build $(BUILDFLAGS) -o bin/ldap-manager-darwin-amd64 .
	@CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build $(BUILDFLAGS) -o bin/ldap-manager-windows-amd64.exe .
	@echo "$(GREEN)✓ Release binaries built$(RESET)"

## Test: Run comprehensive test suite with coverage
test:
	@echo "$(BLUE)Running comprehensive test suite...$(RESET)"
	@./scripts/test.sh

## Test Quick: Run tests without coverage reporting
test-quick:
	@echo "$(BLUE)Running quick tests...$(RESET)"
	@go test -v ./...

## Test Short: Run tests without race detection  
test-short:
	@echo "$(BLUE)Running short tests...$(RESET)"
	@go test -short ./...

## Benchmark: Run performance benchmarks
benchmark:
	@echo "$(BLUE)Running benchmarks...$(RESET)"
	@go test -bench=. -benchmem ./...

## Lint: Run all linting and static analysis
lint: lint-go lint-security lint-format lint-complexity
	@echo "$(GREEN)✓ All linting checks passed$(RESET)"

## Lint Go: Run comprehensive Go linting
lint-go:
	@echo "$(BLUE)Running golangci-lint...$(RESET)"
	@golangci-lint run --config .golangci.yml ./...

## Lint Security: Run security vulnerability checks  
lint-security:
	@echo "$(BLUE)Running security checks...$(RESET)"
	@govulncheck ./...

## Lint Format: Check code formatting
lint-format:
	@echo "$(BLUE)Checking code formatting...$(RESET)"
	@test -z "$$(gofumpt -l .)" || (echo "$(RED)Code not formatted properly$(RESET)" && gofumpt -l . && exit 1)
	@goimports -d . | diff /dev/null - || (echo "$(RED)Imports not formatted properly$(RESET)" && exit 1)

## Lint Complexity: Check code complexity
lint-complexity:
	@echo "$(BLUE)Checking code complexity...$(RESET)"
	@gocyclo -over 10 .

## Fix: Auto-fix formatting and imports
fix:
	@echo "$(BLUE)Fixing code formatting...$(RESET)"
	@gofumpt -w .
	@goimports -w .
	@echo "$(GREEN)✓ Code formatting fixed$(RESET)"

## Dev: Start development server with hot reload
dev:
	@echo "$(BLUE)Starting development server...$(RESET)"
	@pnpm dev

## Docker: Build Docker image
docker:
	@echo "$(BLUE)Building Docker image...$(RESET)"
	@docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	@echo "$(GREEN)✓ Docker image built: $(DOCKER_IMAGE):$(DOCKER_TAG)$(RESET)"

## Docker Run: Run Docker container locally
docker-run: docker
	@echo "$(BLUE)Running Docker container...$(RESET)"
	@docker run -p 3000:3000 --env-file .env.example $(DOCKER_IMAGE):$(DOCKER_TAG)

## Clean: Remove build artifacts and caches
clean:
	@echo "$(BLUE)Cleaning build artifacts...$(RESET)"
	@rm -rf bin/
	@rm -f coverage.out coverage.html
	@rm -rf node_modules/.cache
	@go clean -cache -testcache -modcache
	@echo "$(GREEN)✓ Clean complete$(RESET)"

## Check: Run all quality checks (lint + test)
check: lint test
	@echo "$(GREEN)✓ All quality checks passed$(RESET)"

## Release: Prepare release (check + build-release)
release: check build-release
	@echo "$(GREEN)✓ Release ready$(RESET)"

## Deps: Update dependencies  
deps:
	@echo "$(BLUE)Updating dependencies...$(RESET)"
	@go get -u ./...
	@go mod tidy
	@pnpm update
	@echo "$(GREEN)✓ Dependencies updated$(RESET)"

## Info: Display build information
info:
	@echo "$(BLUE)Build Information:$(RESET)"
	@echo "  Version:       $(VERSION)"
	@echo "  Commit Hash:   $(COMMIT_HASH)"
	@echo "  Build Time:    $(BUILD_TIME)"
	@echo "  Go Version:    $(GO_VERSION)"
	@echo "  Package:       $(PACKAGE)"
	@echo "  Docker Image:  $(DOCKER_IMAGE):$(DOCKER_TAG)"

## Serve: Serve the application (requires build first)
serve: build
	@echo "$(BLUE)Starting LDAP Manager...$(RESET)"
	@./bin/ldap-manager

# Aliases for common commands
install: setup
fmt: fix
check-all: check