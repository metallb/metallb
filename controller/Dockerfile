# syntax=docker/dockerfile:1.2

FROM --platform=$BUILDPLATFORM docker.io/golang:1.17 AS builder
ARG GIT_COMMIT=dev
ARG GIT_BRANCH=dev

WORKDIR $GOPATH/go.universe.tf/metallb
# Caching dependencies
COPY go.mod go.sum ./
RUN go mod download

COPY controller/*.go .
COPY internal internal
COPY api api

# Cache builds directory for faster rebuild
RUN --mount=type=cache,target=/root/.cache/go-build \
  --mount=type=cache,target=/go/pkg \
  CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOARM=6 \
  go build -v -o /build/controller \
  -ldflags "-X 'go.universe.tf/metallb/internal/version.gitCommit=${GIT_COMMIT}' -X 'go.universe.tf/metallb/internal/version.gitBranch=${GIT_BRANCH}'"

FROM docker.io/alpine:latest

COPY --from=builder /build/controller /controller
COPY LICENSE /

LABEL org.opencontainers.image.authors="metallb" \
  org.opencontainers.image.url="https://github.com/metallb/metallb" \
  org.opencontainers.image.documentation="https://metallb.universe.tf" \
  org.opencontainers.image.source="https://github.com/metallb/metallb" \
  org.opencontainers.image.vendor="metallb" \
  org.opencontainers.image.licenses="Apache-2.0" \
  org.opencontainers.image.description="Metallb Controller" \
  org.opencontainers.image.title="controller" \
  org.opencontainers.image.base.name="docker.io/alpine:latest"

ENTRYPOINT ["/controller"]
