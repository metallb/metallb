FROM docker.io/frrouting/frr:v7.5.1

COPY frr-reloader/frr-reloader.sh /frr-reloader.sh
COPY LICENSE /
ENTRYPOINT [ "/frr-reloader.sh" ]
