FROM alpine:latest

ADD frr-reloader.sh /frr-reloader.sh
ADD frr-metrics /frr-metrics
ADD speaker /speaker

# When running as non root and building in an environment that `umask` masks out
# '+x' for others, it won't be possible to execute. Make sure all can execute,
# just in case
RUN chmod a+x /speaker /frr-reloader.sh

ENTRYPOINT ["/speaker"]
