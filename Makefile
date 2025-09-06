# LDAP Manager - Development Makefile
# Provides comprehensive development, testing, and quality assurance tools

# Version and build information
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT_HASH ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
PACKAGE := github.com/netresearch/ldap-manager/internal/version

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

# Coverage settings
COVERAGE_THRESHOLD := 80
COVERAGE_DIR := coverage-reports
COVERAGE_FILE := coverage.out
HTML_COVERAGE_FILE := $(COVERAGE_DIR)/coverage.html

.PHONY: help setup build test lint clean dev docker docker-dev docker-test docker-lint docker-check docker-shell docker-clean

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
	@mkdir -p bin
	@CGO_ENABLED=0 go build $(BUILDFLAGS) -o bin/ldap-manager ./cmd/ldap-manager
	@echo "$(GREEN)✓ Build complete: bin/ldap-manager$(RESET)"

## Build Assets: Build CSS and template assets
build-assets:
	@echo "$(BLUE)Building assets...$(RESET)"
	@pnpm build:assets

## Build Release: Build optimized release binary
build-release: build-assets
	@echo "$(BLUE)Building release binary...$(RESET)"
	@mkdir -p bin
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(BUILDFLAGS) -o bin/ldap-manager-linux-amd64 ./cmd/ldap-manager
	@CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build $(BUILDFLAGS) -o bin/ldap-manager-darwin-amd64 ./cmd/ldap-manager
	@CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build $(BUILDFLAGS) -o bin/ldap-manager-windows-amd64.exe ./cmd/ldap-manager
	@echo "$(GREEN)✓ Release binaries built$(RESET)"

## Test: Run comprehensive test suite with coverage
test:
	@echo "$(BLUE)Running comprehensive test suite...$(RESET)"
	@mkdir -p $(COVERAGE_DIR)
	@if go test -v -race -coverprofile=$(COVERAGE_FILE) -covermode=atomic ./...; then \
		echo "$(GREEN)✅ All tests passed$(RESET)"; \
	else \
		echo "$(RED)❌ Some tests failed$(RESET)"; \
		exit 1; \
	fi
	@if [ -f $(COVERAGE_FILE) ]; then \
		go tool cover -html=$(COVERAGE_FILE) -o $(HTML_COVERAGE_FILE); \
		echo "$(GREEN)✅ HTML coverage report generated: $(HTML_COVERAGE_FILE)$(RESET)"; \
		COVERAGE=$$(go tool cover -func=$(COVERAGE_FILE) | grep "^total:" | awk '{print $$3}' | sed 's/%//'); \
		echo "Coverage Summary:" > $(COVERAGE_DIR)/summary.txt; \
		echo "Total Coverage: $${COVERAGE}%" >> $(COVERAGE_DIR)/summary.txt; \
		echo "Threshold: $(COVERAGE_THRESHOLD)%" >> $(COVERAGE_DIR)/summary.txt; \
		echo "$(BLUE)ℹ️ Coverage: $${COVERAGE}%$(RESET)"; \
		if [ "$${COVERAGE%.*}" -ge "$(COVERAGE_THRESHOLD)" ]; then \
			echo "$(GREEN)✅ Coverage threshold met ($${COVERAGE}% >= $(COVERAGE_THRESHOLD)%)$(RESET)"; \
		else \
			echo "$(YELLOW)⚠️ Coverage below threshold ($${COVERAGE}% < $(COVERAGE_THRESHOLD)%)$(RESET)"; \
		fi; \
	else \
		echo "$(RED)❌ Coverage file not found$(RESET)"; \
		exit 1; \
	fi

## Test Quick: Run tests without coverage reporting
test-quick:
	@echo "$(BLUE)Running quick tests...$(RESET)"
	@go test -v ./...

## Test Short: Run tests without race detection  
test-short:
	@echo "$(BLUE)Running short tests...$(RESET)"
	@go test -short ./...

## Test Race: Run race detection tests
test-race:
	@echo "$(BLUE)Running race detection tests...$(RESET)"
	@if go test -race -short ./...; then \
		echo "$(GREEN)✅ No race conditions detected$(RESET)"; \
	else \
		echo "$(RED)❌ Race conditions detected$(RESET)"; \
		exit 1; \
	fi

## Benchmark: Run performance benchmarks
benchmark:
	@echo "$(BLUE)Running benchmarks...$(RESET)"
	@mkdir -p $(COVERAGE_DIR)
	@go test -bench=. -benchmem -run=^$ ./... > $(COVERAGE_DIR)/benchmarks.txt
	@echo "$(GREEN)✅ Benchmarks completed$(RESET)"

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

## Docker Dev: Build development container with all tools
docker-dev-build:
	@echo "$(BLUE)Building development container...$(RESET)"
	@docker compose build ldap-manager-dev
	@echo "$(GREEN)✓ Development container built$(RESET)"

## Docker Dev: Start development environment with live reload
docker-dev: docker-dev-build
	@echo "$(BLUE)Starting development environment...$(RESET)"
	@echo "$(YELLOW)Starting LDAP server...$(RESET)"
	@docker compose up -d openldap phpldapadmin
	@sleep 5
	@echo "$(YELLOW)Starting development container...$(RESET)"
	@docker compose --profile dev up ldap-manager-dev

## Docker Test: Run tests in container
docker-test:
	@echo "$(BLUE)Running tests in container...$(RESET)"
	@docker compose up -d openldap
	@sleep 5
	@docker compose --profile test run --rm ldap-manager-test
	@if [ $$? -eq 0 ]; then \
		echo "$(GREEN)✅ All tests passed!$(RESET)"; \
	else \
		echo "$(RED)❌ Some tests failed!$(RESET)"; \
		exit 1; \
	fi

## Docker Lint: Run linter in container
docker-lint:
	@echo "$(BLUE)Running linter in container...$(RESET)"
	@docker compose --profile test run --rm ldap-manager-test sh -c "make lint"
	@if [ $$? -eq 0 ]; then \
		echo "$(GREEN)✅ Linting passed!$(RESET)"; \
	else \
		echo "$(RED)❌ Linting failed!$(RESET)"; \
		exit 1; \
	fi

## Docker Check: Run all quality checks in container
docker-check:
	@echo "$(BLUE)Running all quality checks in container...$(RESET)"
	@docker compose up -d openldap
	@sleep 5
	@docker compose --profile test run --rm ldap-manager-test
	@if [ $$? -eq 0 ]; then \
		echo "$(GREEN)✅ All quality checks passed!$(RESET)"; \
	else \
		echo "$(RED)❌ Some quality checks failed!$(RESET)"; \
		exit 1; \
	fi

## Docker Shell: Open shell in development container
docker-shell:
	@echo "$(BLUE)Opening shell in development container...$(RESET)"
	@docker compose up -d openldap
	@sleep 2
	@docker compose --profile dev run --rm ldap-manager-dev sh

## Docker Clean: Clean up containers and volumes
docker-clean:
	@echo "$(YELLOW)⚠️ This will stop all containers and remove volumes. Continue? [y/N]$(RESET)"
	@read -r response; \
	if [ "$$response" = "y" ] || [ "$$response" = "Y" ]; then \
		echo "$(BLUE)Cleaning up containers and volumes...$(RESET)"; \
		docker compose down -v; \
		docker system prune -f; \
		echo "$(GREEN)✓ Docker cleanup completed$(RESET)"; \
	else \
		echo "$(YELLOW)Docker cleanup cancelled$(RESET)"; \
	fi

## Docker Logs: Show logs from development container
docker-logs:
	@echo "$(BLUE)Showing logs from development container...$(RESET)"
	@docker compose logs -f ldap-manager-dev

## Clean: Remove build artifacts and caches
clean:
	@echo "$(BLUE)Cleaning build artifacts...$(RESET)"
	@rm -rf bin/
	@rm -f coverage.out ldap-manager
	@rm -rf $(COVERAGE_DIR)
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