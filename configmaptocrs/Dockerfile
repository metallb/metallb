# syntax=docker/dockerfile:1.2

FROM --platform=$BUILDPLATFORM docker.io/golang:1.23.6 AS builder
WORKDIR $GOPATH/go.universe.tf/metallb

RUN --mount=type=cache,target=/go/pkg/mod/ \
  --mount=type=bind,source=go.sum,target=go.sum \
  --mount=type=bind,source=go.mod,target=go.mod \
  go mod download -x

ARG GIT_COMMIT=dev
ARG GIT_BRANCH=dev
ARG TARGETARCH
ARG TARGETOS
ARG TARGETPLATFORM

# Extract variant
# have to manually convert as building the different arms can cause issues
RUN case ${TARGETPLATFORM} in \
  "linux/arm/v6") export VARIANT="6" ;; \
  "linux/arm/v7") export VARIANT="7" ;; \
  *) export VARIANT="" ;; \
  esac

RUN --mount=type=cache,target=/root/.cache/go-build \
  --mount=type=cache,target=/go/pkg/mod \
  --mount=type=bind,source=go.sum,target=go.sum \
  --mount=type=bind,source=go.mod,target=go.mod \
  --mount=type=bind,source=internal,target=internal \
  --mount=type=bind,source=api,target=api \
  --mount=type=bind,source=configmaptocrs,target=configmaptocrs \
  CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOARM=$VARIANT \
  go build -v -o /build/configmaptocrs \
  -ldflags "-X 'go.universe.tf/metallb/internal/version.gitCommit=${GIT_COMMIT}' -X 'go.universe.tf/metallb/internal/version.gitBranch=${GIT_BRANCH}'" \
  ./configmaptocrs

FROM gcr.io/distroless/static:latest

COPY --from=builder /build/configmaptocrs /configmaptocrs
COPY LICENSE /

LABEL org.opencontainers.image.authors="metallb" \
  org.opencontainers.image.url="https://github.com/metallb/metallb" \
  org.opencontainers.image.documentation="https://metallb.universe.tf" \
  org.opencontainers.image.source="https://github.com/metallb/metallb" \
  org.opencontainers.image.vendor="metallb" \
  org.opencontainers.image.licenses="Apache-2.0" \
  org.opencontainers.image.description="Metallb Configmap to CRs converter" \
  org.opencontainers.image.title="configmap to crs" \
  org.opencontainers.image.base.name="gcr.io/distroless/static:latest"

ENTRYPOINT ["/configmaptocrs"]
