# This dockerfile is not meant to be used directly. Instead, use `fab
# TODO` in the root of the metallb repository.

FROM alpine:latest

RUN echo http://nl.alpinelinux.org/alpine/edge/testing >> /etc/apk/repositories
RUN apk --update --no-cache add bird quagga iptables tcpdump wget tar
ADD test-bgp-router /test-bgp-router

ENTRYPOINT ["/test-bgp-router"]
