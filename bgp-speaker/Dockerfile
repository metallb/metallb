FROM alpine:latest
MAINTAINER Miek Gieben <miek@miek.nl> @miekg
ADD bgp-speaker /bgp-speaker

EXPOSE 8080
ENTRYPOINT ["/bgp-speaker", "--logtostderr"]
