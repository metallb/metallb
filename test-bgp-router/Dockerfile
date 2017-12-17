# This dockerfile is not meant to be used directly. Instead, use `fab
# TODO` in the root of the metallb repository.

FROM alpine:latest

RUN echo http://nl.alpinelinux.org/alpine/edge/testing >> /etc/apk/repositories
RUN apk --update --no-cache add bird quagga iptables tcpdump wget tar
#RUN wget -O /gobgp.tgz https://github.com/osrg/gobgp/releases/download/v1.26/gobgp_1.26_linux_amd64.tar.gz
#RUN cd / && tar zxvf /gobgp.tgz
ADD test-bgp-router /test-bgp-router

ENTRYPOINT ["/test-bgp-router"]
EXPOSE 8080
