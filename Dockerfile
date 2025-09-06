FROM --platform=$BUILDPLATFORM node:22 AS frontend-builder
WORKDIR /build
RUN npm i -g pnpm

COPY package.json .
COPY pnpm-lock.yaml .
RUN pnpm i

COPY . .
RUN pnpm css:build

FROM golang:1.25.1-alpine AS backend-builder
WORKDIR /build
RUN apk add git

COPY ./go.mod .
COPY ./go.sum .
RUN go mod download
RUN go install github.com/a-h/templ/cmd/templ@v0.3.943

COPY . .

COPY --from=frontend-builder /build/internal/web/static/styles.css /build/internal/web/static/styles.css
RUN templ generate
RUN \
  PACKAGE="github.com/netresearch/ldap-manager/internal" && \
  VERSION="$(git describe --tags --always --abbrev=0 --match='v[0-9]*.[0-9]*.[0-9]*' 2> /dev/null | sed 's/^.//')" && \
  COMMIT_HASH="$(git rev-parse --short HEAD)" && \
  BUILD_TIMESTAMP=$(date '+%Y-%m-%dT%H:%M:%S') && \
  CGO_ENABLED=0 go build -o /build/ldap-passwd -ldflags="-s -w -X '${PACKAGE}.Version=${VERSION}' -X '${PACKAGE}.CommitHash=${COMMIT_HASH}' -X '${PACKAGE}.BuildTimestamp=${BUILD_TIMESTAMP}'"

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
  CMD /ldap-passwd --health-check || exit 1

# Use vector form for ENTRYPOINT as recommended by distroless docs
ENTRYPOINT ["/ldap-passwd"]
