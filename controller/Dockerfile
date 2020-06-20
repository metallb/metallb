FROM alpine:latest

ADD controller /controller
# The file is owned by root. Make sure non-root user can execute it
RUN chmod a+x /controller
ENTRYPOINT ["/controller"]
