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
	@test -z "$$(find . -name "*.go" -not -name "*_templ.go" -exec gofumpt -l {} +)" || (echo "$(RED)Code not formatted properly$(RESET)" && find . -name "*.go" -not -name "*_templ.go" -exec gofumpt -l {} + && exit 1)
	@find . -name "*.go" -not -name "*_templ.go" -exec goimports -d {} + | diff /dev/null - || (echo "$(RED)Imports not formatted properly$(RESET)" && exit 1)

## Lint Complexity: Check code complexity
lint-complexity:
	@echo "$(BLUE)Checking code complexity...$(RESET)"
	@find . -name "*.go" -not -path "./vendor/*" -not -name "*_templ.go" -exec gocyclo -over 10 {} \;

## Fix: Auto-fix formatting and imports
fix:
	@echo "$(BLUE)Fixing code formatting...$(RESET)"
	@find . -name "*.go" -not -name "*_templ.go" -exec gofumpt -w {} +
	@find . -name "*.go" -not -name "*_templ.go" -exec goimports -w {} +
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

# ============================================================================
# Application Control Commands
# ============================================================================

## Up: Start all services (LDAP server + app)
up:
	@echo "$(BLUE)Starting all services...$(RESET)"
	@docker compose --profile dev up -d
	@echo "$(GREEN)✓ Services started$(RESET)"
	@echo "$(YELLOW)App: http://localhost:3000$(RESET)"
	@echo "$(YELLOW)phpLDAPadmin: http://localhost:8080$(RESET)"

## Down: Stop all services
down:
	@echo "$(BLUE)Stopping all services...$(RESET)"
	@docker compose down
	@echo "$(GREEN)✓ Services stopped$(RESET)"

## Restart: Restart all services
restart: down up
	@echo "$(GREEN)✓ Services restarted$(RESET)"

## Start: Start services without rebuilding
start:
	@echo "$(BLUE)Starting services...$(RESET)"
	@docker compose --profile dev start
	@echo "$(GREEN)✓ Services started$(RESET)"

## Stop: Stop services without removing containers
stop:
	@echo "$(BLUE)Stopping services...$(RESET)"
	@docker compose stop
	@echo "$(GREEN)✓ Services stopped$(RESET)"

## Logs: Show logs from all services
logs:
	@docker compose logs -f

## Logs App: Show logs from app only
logs-app:
	@docker compose logs -f ldap-manager-dev

## Logs LDAP: Show logs from LDAP server
logs-ldap:
	@docker compose logs -f openldap

## PS: Show running services
ps:
	@docker compose ps

## Shell App: Open shell in app container
shell-app:
	@echo "$(BLUE)Opening shell in app container...$(RESET)"
	@docker compose exec ldap-manager-dev sh || docker compose run --rm ldap-manager-dev sh

## Shell LDAP: Open shell in LDAP container
shell-ldap:
	@echo "$(BLUE)Opening shell in LDAP container...$(RESET)"
	@docker compose exec openldap bash

## Rebuild: Rebuild and restart services
rebuild:
	@echo "$(BLUE)Rebuilding services...$(RESET)"
	@docker compose build ldap-manager-dev
	@docker compose --profile dev up -d --force-recreate ldap-manager-dev
	@echo "$(GREEN)✓ Services rebuilt and restarted$(RESET)"

## Fresh: Clean everything and start fresh
fresh: docker-clean
	@echo "$(BLUE)Starting fresh environment...$(RESET)"
	@docker compose build ldap-manager-dev
	@docker compose --profile dev up -d
	@echo "$(GREEN)✓ Fresh environment ready$(RESET)"

# ============================================================================
# Development Workflow Commands
# ============================================================================

## Watch: Watch and rebuild assets on change
watch:
	@echo "$(BLUE)Watching for changes...$(RESET)"
	@pnpm dev

## CSS Build: Build CSS only
css:
	@echo "$(BLUE)Building CSS...$(RESET)"
	@pnpm css:build:prod
	@echo "$(GREEN)✓ CSS built$(RESET)"

## CSS Watch: Watch and rebuild CSS
css-watch:
	@echo "$(BLUE)Watching CSS...$(RESET)"
	@pnpm css:dev

## Templates: Generate Go templates from .templ files
templates:
	@echo "$(BLUE)Generating templates...$(RESET)"
	@pnpm templ:build
	@echo "$(GREEN)✓ Templates generated$(RESET)"

## Templates Watch: Watch and regenerate templates
templates-watch:
	@echo "$(BLUE)Watching templates...$(RESET)"
	@pnpm templ:dev

## Format Go: Format Go code
format-go:
	@echo "$(BLUE)Formatting Go code...$(RESET)"
	@gofumpt -w .
	@goimports -w .
	@echo "$(GREEN)✓ Go code formatted$(RESET)"

## Format JS: Format JavaScript/JSON/CSS
format-js:
	@echo "$(BLUE)Formatting JS/JSON/CSS...$(RESET)"
	@pnpm prettier --write .
	@echo "$(GREEN)✓ JS/JSON/CSS formatted$(RESET)"

## Format All: Format all code
format-all: format-go format-js
	@echo "$(GREEN)✓ All code formatted$(RESET)"

# ============================================================================
# Database/LDAP Management
# ============================================================================

## LDAP Reset: Reset LDAP database
ldap-reset:
	@echo "$(YELLOW)⚠️ This will delete all LDAP data. Continue? [y/N]$(RESET)"
	@read -r response; \
	if [ "$$response" = "y" ] || [ "$$response" = "Y" ]; then \
		echo "$(BLUE)Resetting LDAP database...$(RESET)"; \
		docker compose down openldap; \
		docker volume rm ldap-manager_ldap_data ldap-manager_ldap_config || true; \
		docker compose up -d openldap; \
		echo "$(GREEN)✓ LDAP database reset$(RESET)"; \
	else \
		echo "$(YELLOW)LDAP reset cancelled$(RESET)"; \
	fi

## LDAP Admin: Open phpLDAPadmin in browser
ldap-admin:
	@echo "$(BLUE)Opening phpLDAPadmin...$(RESET)"
	@command -v xdg-open >/dev/null 2>&1 && xdg-open http://localhost:8080 || \
	 command -v open >/dev/null 2>&1 && open http://localhost:8080 || \
	 echo "$(YELLOW)Open manually: http://localhost:8080$(RESET)"

## Sessions Clean: Clean session database
sessions-clean:
	@echo "$(BLUE)Cleaning session database...$(RESET)"
	@rm -f session.bbolt db.bbolt
	@docker compose exec ldap-manager-dev rm -f /app/session.bbolt /app/db.bbolt 2>/dev/null || true
	@echo "$(GREEN)✓ Session database cleaned$(RESET)"

# ============================================================================
# Monitoring & Debugging
# ============================================================================

## Health: Check service health
health:
	@echo "$(BLUE)Checking service health...$(RESET)"
	@echo ""
	@echo "$(YELLOW)LDAP Server:$(RESET)"
	@docker compose exec openldap ldapsearch -x -H ldap://localhost -b "dc=netresearch,dc=local" -D "cn=admin,dc=netresearch,dc=local" -w admin -LLL "(objectClass=*)" dn 2>/dev/null | head -5 && echo "$(GREEN)✓ LDAP healthy$(RESET)" || echo "$(RED)✗ LDAP unhealthy$(RESET)"
	@echo ""
	@echo "$(YELLOW)App Health:$(RESET)"
	@curl -sf http://localhost:3000/health >/dev/null && echo "$(GREEN)✓ App healthy$(RESET)" || echo "$(RED)✗ App unhealthy$(RESET)"
	@echo ""

## Stats: Show resource usage
stats:
	@echo "$(BLUE)Container resource usage:$(RESET)"
	@docker stats --no-stream --format "table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.NetIO}}" $$(docker compose ps -q)

## Inspect: Inspect app container
inspect:
	@docker compose exec ldap-manager-dev sh -c 'echo "=== Environment ===" && env | sort && echo "" && echo "=== Processes ===" && ps aux'

## Debug: Start app in debug mode
debug:
	@echo "$(BLUE)Starting app in debug mode...$(RESET)"
	@docker compose exec ldap-manager-dev go run -ldflags="-X main.debug=true" .

# ============================================================================
# Quick Access URLs
# ============================================================================

## Open: Open app in browser
open:
	@echo "$(BLUE)Opening app...$(RESET)"
	@command -v xdg-open >/dev/null 2>&1 && xdg-open http://localhost:3000 || \
	 command -v open >/dev/null 2>&1 && open http://localhost:3000 || \
	 echo "$(YELLOW)Open manually: http://localhost:3000$(RESET)"

## URLs: Show all service URLs
urls:
	@echo "$(BLUE)Service URLs:$(RESET)"
	@echo "  $(YELLOW)App:$(RESET)              http://localhost:3000"
	@echo "  $(YELLOW)phpLDAPadmin:$(RESET)     http://localhost:8080"
	@echo "  $(YELLOW)LDAP Server:$(RESET)      ldap://localhost:389"
	@echo "  $(YELLOW)LDAPS Server:$(RESET)     ldaps://localhost:636"

# ============================================================================
# Git Workflow
# ============================================================================

## Git Status: Show git status with branch info
git-status:
	@git status
	@echo ""
	@echo "$(BLUE)Branch:$(RESET) $$(git branch --show-current)"
	@echo "$(BLUE)Commits ahead of main:$(RESET) $$(git rev-list --count main..HEAD)"

## Commit: Stage all and commit with message
commit:
	@echo "$(BLUE)Staging changes...$(RESET)"
	@git add -A
	@git status --short
	@echo ""
	@echo "$(YELLOW)Enter commit message:$(RESET)"
	@read -r msg; \
	if [ -n "$$msg" ]; then \
		git commit -m "$$msg"; \
		echo "$(GREEN)✓ Committed$(RESET)"; \
	else \
		echo "$(RED)✗ No commit message provided$(RESET)"; \
	fi

## Push: Push current branch
push:
	@echo "$(BLUE)Pushing to remote...$(RESET)"
	@git push origin $$(git branch --show-current)
	@echo "$(GREEN)✓ Pushed$(RESET)"

# ============================================================================
# Aliases for common commands
# ============================================================================
install: setup
fmt: fix
check-all: check
run: up
dev-start: up
dev-stop: down
app-logs: logs-app