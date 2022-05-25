FROM alpine:latest

# When running as non root and building in an environment that `umask` masks out
# '+x' for others, it won't be possible to execute. Make sure all can execute,
# just in case
COPY --chmod=a+x controller /controller

ENTRYPOINT ["/controller"]
