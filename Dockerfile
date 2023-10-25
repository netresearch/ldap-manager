FROM --platform=amd64 node:20 AS frontend-builder
WORKDIR /build
RUN npm i -g pnpm

COPY package.json .
COPY pnpm-lock.yaml .
RUN pnpm i

COPY . .

RUN pnpm build:assets

FROM golang:1.21-alpine AS backend-builder
WORKDIR /build
RUN apk add git

COPY ./go.mod .
COPY ./go.sum .
RUN go mod download

COPY . .

COPY --from=frontend-builder /build/internal/web/static/styles.css /build/internal/web/static/styles.css
RUN \
  PACKAGE="github.com/netresearch/ldap-manager/internal" && \
  VERSION="$(git describe --tags --always --abbrev=0 --match='v[0-9]*.[0-9]*.[0-9]*' 2> /dev/null | sed 's/^.//')" && \
  COMMIT_HASH="$(git rev-parse --short HEAD)" && \
  BUILD_TIMESTAMP=$(date '+%Y-%m-%dT%H:%M:%S') && \
  CGO_ENABLED=0 go build -o /build/ldap-passwd -ldflags="-s -w -X '${PACKAGE}.Version=${VERSION}' -X '${PACKAGE}.CommitHash=${COMMIT_HASH}' -X '${PACKAGE}.BuildTimestamp=${BUILD_TIMESTAMP}'"

FROM alpine:3 AS runner

EXPOSE 3000

COPY --from=backend-builder /build/ldap-passwd /usr/local/bin/ldap-passwd

ENTRYPOINT [ "/usr/local/bin/ldap-passwd" ]
