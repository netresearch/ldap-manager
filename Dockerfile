# syntax=docker/dockerfile:1.24

# --- binary-selector stage (production / CI) -----------------------------
# release.yml's binaries matrix (build-go-attest.yml) cross-compiles Go
# binaries with frontend assets embedded via go:embed, publishes them as
# release assets, and the container job downloads them back into bin/.
# This stage picks the correct pre-built binary for TARGETARCH /
# TARGETVARIANT — no `go build`, no `bun install`, no `templ generate`
# happens inside Docker.
FROM alpine:3.23.4 AS binary-selector

ARG TARGETARCH
ARG TARGETVARIANT

COPY bin/ldap-manager-linux-* /tmp/

RUN set -eux; \
    case "${TARGETARCH}" in \
        arm)              BINARY="ldap-manager-linux-arm${TARGETVARIANT}" ;; \
        386|amd64|arm64)  BINARY="ldap-manager-linux-${TARGETARCH}" ;; \
        *) echo "Unsupported architecture: ${TARGETARCH}" >&2; exit 1 ;; \
    esac; \
    cp "/tmp/${BINARY}" /usr/bin/ldap-manager; \
    chmod +x /usr/bin/ldap-manager

# --- dev stage (local development containers) ----------------------------
# Used by compose.yml's ldap-manager-dev / ldap-manager-test services.
# Not part of the release pipeline — `docker buildx build --target=dev`
# only locally. Source code is bind-mounted at runtime.
FROM golang:1.26.4-alpine AS dev

SHELL ["/bin/ash", "-eo", "pipefail", "-c"]
WORKDIR /app

RUN apk add --no-cache git make curl bash && \
    curl -fsSL https://bun.sh/install | bash && \
    ln -s /root/.bun/bin/bun /usr/local/bin/bun && \
    ln -s /root/.bun/bin/bunx /usr/local/bin/bunx

COPY go.mod go.sum package.json bun.lock ./

RUN --mount=type=cache,target=/go/pkg/mod,sharing=locked \
    --mount=type=cache,target=/root/.bun/install/cache,sharing=locked \
    go mod download && \
    bun install --frozen-lockfile

CMD ["sh"]

# --- runner stage (production runtime) -----------------------------------
# Distroless nonroot: no shell, no package manager, minimal attack surface.
FROM gcr.io/distroless/static-debian12:nonroot AS runner

LABEL org.opencontainers.image.title="LDAP Manager" \
      org.opencontainers.image.description="Web-based LDAP user and group management tool" \
      org.opencontainers.image.vendor="Netresearch DTT GmbH" \
      org.opencontainers.image.source="https://github.com/netresearch/ldap-manager" \
      org.opencontainers.image.licenses="MIT"

# Build metadata (populated by docker/metadata-action in build-container.yml).
ARG BUILD_DATE
ARG VCS_REF
LABEL org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.revision="${VCS_REF}"

EXPOSE 3000

COPY --from=binary-selector \
     --chown=nonroot:nonroot \
     --chmod=555 \
     /usr/bin/ldap-manager /ldap-manager

USER nonroot:nonroot

# Uses the binary's --health-check flag (works under distroless: no shell
# needed). See cmd/ldap-manager/main.go:runHealthCheck.
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD ["/ldap-manager", "--health-check"]

ENTRYPOINT ["/ldap-manager"]
