# This dockerfile is not meant to be used directly. Instead, use `fab
# TODO` in the root of the metallb repository.

# We have to build gobgpd from scratch here, because the binary
# releases are linked against glibc (alpine uses musl), and the
# official docker image for gobgpd uses debian jessie, and therefore
# is 400MiB.
#
# Building this is slow, but it's a one-time hit because after that,
# the entire builder image is cached and we just grab the same binary
# over and over again.
FROM alpine:latest AS builder

ENV GOPATH=/go
ENV GOBIN=/

RUN apk --update upgrade
RUN apk add git go musl-dev linux-headers
RUN go get github.com/golang/dep/cmd/dep
RUN go get -d github.com/osrg/gobgp/gobgpd
WORKDIR $GOPATH/src/github.com/osrg/gobgp
RUN /dep ensure
RUN go install github.com/osrg/gobgp/gobgpd

FROM alpine:latest

RUN echo http://nl.alpinelinux.org/alpine/edge/testing >> /etc/apk/repositories && \
    apk --update --no-cache upgrade && \
    apk --no-cache add bird quagga iptables tcpdump wget tar
COPY --from=builder /gobgpd /gobgpd
ADD test-bgp-router /test-bgp-router

ENTRYPOINT ["/test-bgp-router"]
EXPOSE 8080
