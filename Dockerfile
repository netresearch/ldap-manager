# syntax=docker/dockerfile:1.27

# --- binary-selector stage (production / CI) -----------------------------
# release.yml's binaries matrix (build-go-attest.yml) cross-compiles Go
# binaries with frontend assets embedded via go:embed, publishes them as
# release assets, and the container job downloads them back into bin/.
# This stage picks the correct pre-built binary for TARGETARCH /
# TARGETVARIANT — no `go build` and no `templ generate`
# happens inside Docker.
#
# Third-party base images are pinned by the digest of their multi-arch index,
# so a re-pushed tag cannot change what builds. Renovate and Dependabot
# update tag and digest together.
FROM alpine:3.24.1@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b AS binary-selector

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

# --- runner stage (production runtime) -----------------------------------
# Distroless nonroot: no shell, no package manager, minimal attack surface.
FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab AS runner

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
     --chown=65532:65532 \
     --chmod=555 \
     /usr/bin/ldap-manager /ldap-manager

# 65532 is distroless's nonroot user. The numeric form lets Kubernetes'
# runAsNonRoot verify it, which it cannot do for a user name.
USER 65532:65532

# Uses the binary's --health-check flag (works under distroless: no shell
# needed). See cmd/ldap-manager/main.go:runHealthCheck.
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD ["/ldap-manager", "--health-check"]

ENTRYPOINT ["/ldap-manager"]
