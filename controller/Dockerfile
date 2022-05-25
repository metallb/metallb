FROM alpine:latest

# When running as non root and building in an environment that `umask` masks out
# '+x' for others, it won't be possible to execute. Make sure all can execute,
# just in case
COPY --chmod=a+x controller /controller

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
