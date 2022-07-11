# syntax=docker/dockerfile:1.2

FROM --platform=$BUILDPLATFORM docker.io/golang:1.18.3 AS builder
ARG GIT_COMMIT=dev
ARG GIT_BRANCH=dev
WORKDIR $GOPATH/go.universe.tf/metallb

# Cache the downloads
COPY go.mod go.sum ./
RUN go mod download

# Copy speaker
COPY speaker/*.go speaker/
# Copy frr-metrics
COPY frr-metrics ./frr-metrics/
# COPY internals
COPY internal internal
COPY api api

ARG TARGETARCH
ARG TARGETOS
ARG TARGETPLATFORM

# have to manually convert as building the different arms can cause issues
# Extract variant
RUN case ${TARGETPLATFORM} in \
  "linux/arm/v6") export VARIANT="6" ;; \
  "linux/arm/v7") export VARIANT="7" ;; \
  *) export VARIANT="" ;; \
  esac

# Cache builds directory for faster rebuild
RUN --mount=type=cache,target=/root/.cache/go-build \
  --mount=type=cache,target=/go/pkg \
  # build frr metrics
  CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOARM=$VARIANT \
  go build -v -o /build/frr-metrics \
  -ldflags "-X 'go.universe.tf/metallb/internal/version.gitCommit=${GIT_COMMIT}' -X 'go.universe.tf/metallb/internal/version.gitBranch=${GIT_BRANCH}'" \
  frr-metrics/exporter.go \
  && \
  # build speaker
  CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOARM=$VARIANT \
  go build -v -o /build/speaker \
  -ldflags "-X 'go.universe.tf/metallb/internal/version.gitCommit=${GIT_COMMIT}' -X 'go.universe.tf/metallb/internal/version.gitBranch=${GIT_BRANCH}'" \
  go.universe.tf/metallb/speaker

FROM docker.io/alpine:latest


COPY --from=builder /build/speaker /speaker
COPY --from=builder /build/frr-metrics /frr-metrics
COPY frr-reloader/frr-reloader.sh /frr-reloader.sh
COPY LICENSE /

LABEL org.opencontainers.image.authors="metallb" \
  org.opencontainers.image.url="https://github.com/metallb/metallb" \
  org.opencontainers.image.documentation="https://metallb.universe.tf" \
  org.opencontainers.image.source="https://github.com/metallb/metallb" \
  org.opencontainers.image.vendor="metallb" \
  org.opencontainers.image.licenses="Apache-2.0" \
  org.opencontainers.image.description="Metallb speaker" \
  org.opencontainers.image.title="speaker" \
  org.opencontainers.image.base.name="docker.io/alpine:latest"

ENTRYPOINT ["/speaker"]
