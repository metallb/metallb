FROM alpine:latest

ADD controller /controller
EXPOSE 8080
ENTRYPOINT ["/controller", "--logtostderr"]
