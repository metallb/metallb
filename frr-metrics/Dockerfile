# syntax=docker/dockerfile:1.2

FROM --platform=$BUILDPLATFORM docker.io/golang:1.17 AS builder
ARG GIT_COMMIT=dev
ARG GIT_BRANCH=dev
WORKDIR $GOPATH/go.universe.tf/metallb

# Cache the downloads
COPY go.mod go.sum ./
RUN go mod download

# Copy frr-metrics
COPY frr-metrics ./frr-metrics/
# COPY internals
COPY internal internal
COPY api api


RUN --mount=type=cache,target=/root/.cache/go-build \
  --mount=type=cache,target=/go/pkg \
  # build frr metrics
  CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOARM=6 \
  go build -v -o /build/frr-metrics \
  -ldflags "-X 'go.universe.tf/metallb/internal/version.gitCommit=${GIT_COMMIT}' -X 'go.universe.tf/metallb/internal/version.gitBranch=${GIT_BRANCH}'" \
  frr-metrics/exporter.go

FROM alpine:latest

COPY --from=builder /build/frr-metrics /frr-metrics
COPY LICENSE /
ENTRYPOINT ["/frr-metrics"]
