FROM --platform=$BUILDPLATFORM node:22 AS frontend-builder
WORKDIR /build
RUN npm i -g pnpm

COPY package.json .
COPY pnpm-lock.yaml .
RUN pnpm i

COPY . .
RUN pnpm css:build

# Development stage with all tools for linting, testing, and development
FROM golang:1.25.1-alpine AS dev

# Set shell with pipefail for safe pipe operations in Alpine
SHELL ["/bin/ash", "-eo", "pipefail", "-c"]

WORKDIR /app

# Install system dependencies for development
RUN apk add --no-cache \
    git=2.49.1-r0 \
    make=4.4.1-r3 \
    curl=8.14.1-r1

# Install Node.js and PNPM for frontend development
RUN apk add --no-cache nodejs=22.16.0-r2 npm=11.3.0-r1 && \
    npm i -g pnpm

# Install Go development tools
RUN go install github.com/a-h/templ/cmd/templ@v0.3.943 && \
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest && \
    go install golang.org/x/tools/cmd/goimports@latest && \
    go install mvdan.cc/gofumpt@latest

# Copy dependency files first for better caching
COPY go.mod go.sum ./
RUN go mod download

COPY package.json pnpm-lock.yaml ./
RUN pnpm install

# Copy source code
COPY . .

# Generate templates and build CSS
RUN pnpm css:build && templ generate

# Set default command for development
CMD ["sh"]

# Production builder stage
FROM golang:1.25.1-alpine AS backend-builder

# Set shell with pipefail for safe pipe operations in Alpine
SHELL ["/bin/ash", "-eo", "pipefail", "-c"]

WORKDIR /build
RUN apk add --no-cache git=2.49.1-r0

COPY ./go.mod .
COPY ./go.sum .
RUN go mod download && \
  go install github.com/a-h/templ/cmd/templ@v0.3.943

COPY . .

COPY --from=frontend-builder /build/internal/web/static/styles.css /build/internal/web/static/styles.css
RUN templ generate

RUN \
  PACKAGE="github.com/netresearch/ldap-manager/internal/version" && \
  VERSION="$(git describe --tags --always --abbrev=0 --match='v[0-9]*.[0-9]*.[0-9]*' 2>/dev/null | sed 's/^.//')" && \
  COMMIT_HASH="$(git rev-parse --short HEAD)" && \
  BUILD_TIMESTAMP=$(date '+%Y-%m-%dT%H:%M:%S') && \
  CGO_ENABLED=0 go build -o /build/ldap-passwd -ldflags="-s -w -X '${PACKAGE}.Version=${VERSION}' -X '${PACKAGE}.CommitHash=${COMMIT_HASH}' -X '${PACKAGE}.BuildTimestamp=${BUILD_TIMESTAMP}'" ./cmd/ldap-manager

FROM gcr.io/distroless/static-debian12:nonroot AS runner

# Security and metadata labels
LABEL org.opencontainers.image.title="LDAP Manager"
LABEL org.opencontainers.image.description="Web-based LDAP user and group management tool"
LABEL org.opencontainers.image.vendor="Netresearch"
LABEL org.opencontainers.image.source="https://github.com/netresearch/ldap-manager"
LABEL org.opencontainers.image.licenses="MIT"

EXPOSE 3000

# Use COPY with --chown for nonroot user (safer than root)
COPY --from=backend-builder --chown=nonroot:nonroot /build/ldap-passwd /ldap-passwd

# Set user to nonroot for security
USER nonroot:nonroot

# Health check to ensure the application is running
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD ["/ldap-passwd", "--health-check"]

# Use vector form for ENTRYPOINT as recommended by distroless docs
ENTRYPOINT ["/ldap-passwd"]