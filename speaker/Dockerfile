FROM alpine:latest as intermediate

ADD speaker /speaker

# When running as non root and building in an environment that `umask` masks out
# '+x' for others, it won't be possible to execute. Make sure all can execute,
# just in case
RUN chmod a+x /speaker

# Production image
FROM gcr.io/distroless/static:latest
COPY --from=intermediate /speaker /speaker

ENTRYPOINT ["/speaker"]
