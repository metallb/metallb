FROM alpine:latest
MAINTAINER Miek Gieben <miek@miek.nl> @miekg
ADD controller /controller

EXPOSE 8080
ENTRYPOINT ["/controller", "--logtostderr"]
