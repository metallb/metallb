FROM alpine:latest
MAINTAINER Miek Gieben <miek@miek.nl> @miekg
ADD controller /controller
USER nobody:nobody
ENTRYPOINT ["/controller", "--logtostderr"]
