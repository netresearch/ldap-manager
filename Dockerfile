# syntax=docker/dockerfile:1.21
# Enable BuildKit for advanced features like cache mounts

# Frontend builder - builds Tailwind CSS
# Uses Alpine for 80% smaller image size (~200MB vs ~1GB)
FROM --platform=$BUILDPLATFORM node:24-alpine AS frontend-builder
WORKDIR /build

# Enable pnpm via corepack (built into Node 22, no install needed)
RUN corepack enable pnpm

# Copy dependency files first for better layer caching
COPY package.json pnpm-lock.yaml ./

# Install dependencies with cache mount for faster rebuilds
# Cache persists between builds, avoiding re-downloads
# sharing=locked prevents race conditions in parallel builds
RUN --mount=type=cache,target=/root/.local/share/pnpm/store,sharing=locked \
    pnpm install --frozen-lockfile

# Copy only files needed for CSS build to maximize cache efficiency
COPY postcss.config.mjs tailwind.config.js ./
COPY scripts/cache-bust.mjs ./scripts/
COPY internal/web/tailwind.css ./internal/web/
COPY internal/web/templates ./internal/web/templates

# Build CSS
RUN pnpm css:build

# Development stage with all tools for linting, testing, and development
FROM golang:1.25.6-alpine AS dev

# Set shell with pipefail for safe pipe operations in Alpine
SHELL ["/bin/ash", "-eo", "pipefail", "-c"]

WORKDIR /app

# Install system dependencies for development
# and install pnpm globally (corepack not available in Alpine nodejs package)
# Note: Using latest stable versions instead of pinned versions for faster builds
RUN apk add --no-cache \
    git \
    make \
    curl \
    nodejs \
    npm && \
    npm install -g pnpm@10.17.1

# Copy dependency files first for better caching
# Note: Dev tools (templ, golangci-lint, goimports, gofumpt) are declared in go.mod
# and available via `go tool <name>` after go mod download
COPY go.mod go.sum package.json pnpm-lock.yaml ./

# Download Go modules and install Node dependencies with cache mounts
# sharing=locked prevents race conditions in parallel builds
RUN --mount=type=cache,target=/go/pkg/mod,sharing=locked \
    --mount=type=cache,target=/root/.local/share/pnpm/store,sharing=locked \
    go mod download && \
    pnpm install --frozen-lockfile

# Note: Source code is mounted at runtime via compose.yml, not copied here
# The dev container rebuilds CSS/templates on demand with mounted source

# Set default command for development
CMD ["sh"]

# Production builder stage
FROM golang:1.25.6-alpine AS backend-builder

# Set shell with pipefail for safe pipe operations in Alpine
SHELL ["/bin/ash", "-eo", "pipefail", "-c"]

WORKDIR /build

# Install git for version detection during build
RUN apk add --no-cache git

# Copy dependency files first for better layer caching
COPY go.mod go.sum ./

# Download dependencies (templ is declared as a tool in go.mod)
RUN --mount=type=cache,target=/go/pkg/mod,sharing=locked \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

# Copy compiled CSS files and manifest from frontend builder (including hashed versions for cache busting)
# Copy these BEFORE source code to maximize cache hits when only Go code changes
COPY --from=frontend-builder /build/internal/web/static/*.css /build/internal/web/static/
COPY --from=frontend-builder /build/internal/web/static/manifest.json /build/internal/web/static/manifest.json

# Copy source code
COPY . .

# Generate Go templates from .templ files (using go tool for version consistency)
RUN go tool templ generate

# Build production binary with:
# - Version info from git tags
# - Optimizations: -s -w (strip debug info, reduce size)
# - CGO_ENABLED=0 for static linking (required for distroless)
# - Cache mount for faster builds
# - Parallel compilation with GOMAXPROCS
RUN --mount=type=cache,target=/root/.cache/go-build,sharing=locked \
    PACKAGE="github.com/netresearch/ldap-manager/internal/version" && \
    VERSION_RAW="$(git describe --tags --always --abbrev=0 --match='v[0-9]*.[0-9]*.[0-9]*' 2>/dev/null)" && \
    VERSION="${VERSION_RAW#v}" && \
    COMMIT_HASH="$(git rev-parse --short HEAD)" && \
    BUILD_TIMESTAMP=$(date -u '+%Y-%m-%dT%H:%M:%SZ') && \
    CGO_ENABLED=0 go build -p 4 \
      -o /build/ldap-passwd \
      -ldflags="-s -w -X '${PACKAGE}.Version=${VERSION}' -X '${PACKAGE}.CommitHash=${COMMIT_HASH}' -X '${PACKAGE}.BuildTimestamp=${BUILD_TIMESTAMP}'" \
      ./cmd/ldap-manager

# Production runtime - minimal distroless image
# - No shell, package manager, or unnecessary tools (security)
# - Nonroot user by default (security)
# - Static Debian 12 base (~2MB vs ~100MB for alpine)
FROM gcr.io/distroless/static-debian12:nonroot AS runner

# Security and metadata labels
LABEL org.opencontainers.image.title="LDAP Manager"
LABEL org.opencontainers.image.description="Web-based LDAP user and group management tool"
LABEL org.opencontainers.image.vendor="Netresearch"
LABEL org.opencontainers.image.source="https://github.com/netresearch/ldap-manager"
LABEL org.opencontainers.image.licenses="MIT"

# Build metadata for security tracking
ARG BUILD_DATE
ARG VCS_REF
LABEL org.opencontainers.image.created="${BUILD_DATE}"
LABEL org.opencontainers.image.revision="${VCS_REF}"

EXPOSE 3000

# Copy binary with explicit permissions:
# - chown=nonroot:nonroot (run as non-privileged user)
# - chmod=555 (read+execute only, prevents tampering)
COPY --from=backend-builder \
     --chown=nonroot:nonroot \
     --chmod=555 \
     /build/ldap-passwd /ldap-passwd

# Set user to nonroot for security (UID 65532)
USER nonroot:nonroot

# Health check using built-in --health-check flag (works in distroless without shell)
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD ["/ldap-passwd", "--health-check"]

# Use vector form for ENTRYPOINT as recommended by distroless docs
ENTRYPOINT ["/ldap-passwd"]
