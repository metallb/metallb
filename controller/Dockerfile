FROM alpine:latest as intermediate

ADD controller /controller

# When running as non root and building in an environment that `umask` masks out
# '+x' for others, it won't be possible to execute. Make sure all can execute,
# just in case
RUN chmod a+x /controller

# Production image
FROM gcr.io/distroless/static:latest
COPY --from=intermediate /controller /controller

ENTRYPOINT ["/controller"]
