FROM alpine:latest
MAINTAINER Miek Gieben <miek@miek.nl> @miekg
ADD bgp-speaker /bgp-speaker
USER nobody:nobody
ENTRYPOINT ["/bgp-speaker", "--logtostderr"]
