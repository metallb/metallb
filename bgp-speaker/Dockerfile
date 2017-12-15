FROM alpine:latest

ADD bgp-speaker /bgp-speaker
EXPOSE 8080
ENTRYPOINT ["/bgp-speaker", "--logtostderr"]
