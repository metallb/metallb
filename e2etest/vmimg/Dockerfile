FROM golang:1.10 as builder
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    libglib2.0-dev \
    libostree-dev \
    && rm -rf /var/lib/apt/lists/*
RUN go get -u github.com/go-debos/debos/cmd/debos

FROM debian:buster-slim as runner
RUN apt-get update && \
    DEBIAN_FRONTEND=noninteractive \
    apt-get install -y --no-install-recommends \
    busybox \
    ca-certificates \
    dbus \
    debootstrap \
    libostree-1-1 \
    linux-image-amd64 \
    parted \
    qemu-system-x86 \
    qemu-user-static \
    qemu-utils \
    systemd \
    systemd-container \
    && rm -rf /var/lib/apt/lists/*
COPY --from=builder /go/bin/debos /usr/bin/debos
VOLUME /home/debos
VOLUME /home/debos-out
WORKDIR /home/debos
ENTRYPOINT ["/usr/bin/debos"]
