FROM alpine:latest
MAINTAINER Miek Gieben <miek@miek.nl> @miekg

ADD arp-speaker /arp-speaker
RUN setcap 'cap_net_bind_service=+ep' /arp-speaker

USER nobody:nobody

ENTRYPOINT ["/arp-speaker", "--logtostderr"]
