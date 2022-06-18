# syntax=docker/dockerfile:1.2

FROM --platform=$BUILDPLATFORM docker.io/golang:1.17 AS builder
WORKDIR $GOPATH/go.universe.tf/metallb
# Caching dependencies
COPY go.mod go.sum ./
RUN go mod download

COPY mirror-server/*.go .
RUN --mount=type=cache,target=/root/.cache/go-build \
  --mount=type=cache,target=/go/pkg \
  GOOS=$TARGETOS GOARCH=$TARGETARCH CGO_ENABLED=0 \
  go build -v -o /build/mirror

FROM alpine:latest

COPY --from=builder /build/mirror /mirror
COPY LICENSE /
ENTRYPOINT ["/mirror"]
