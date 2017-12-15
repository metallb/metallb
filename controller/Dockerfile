# This dockerfile is not meant to be used directly. Instead, use `fab
# TODO` in the root of the metallb repository.
FROM alpine:latest

RUN apk --update --no-cache upgrade
ADD controller /controller

ENTRYPOINT ["/controller", "--logtostderr"]
USER nobody:nobody
EXPOSE 8080
