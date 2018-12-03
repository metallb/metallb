FROM dev_vpp_agent as devimg

FROM ubuntu:18.04 as base

RUN apt-get update \
 && apt-get install -y --no-install-recommends \
     # general tools
     inetutils-traceroute \
     iproute2 \
     iputils-ping \
     # vpp requirements
     ca-certificates \
     libapr1 \
     libc6 \
     libmbedcrypto1 \
     libmbedtls10 \
     libmbedx509-0 \
     libnuma1 \
     openssl \
     # other
     ipsec-tools \
     python \
     supervisor \
 && rm -rf /var/lib/apt/lists/*

# install vpp
COPY --from=devimg \
    /opt/vpp-agent/dev/vpp/build-root/vpp_*.deb \
    /opt/vpp-agent/dev/vpp/build-root/vpp-lib*.deb \
    /opt/vpp-agent/dev/vpp/build-root/vpp-plugins*.deb \
 /opt/vpp/

RUN cd /opt/vpp/ \
 && dpkg -i vpp_*.deb vpp-lib_*.deb vpp-plugins_*.deb \
 && rm vpp*.deb

FROM scratch

COPY --from=base / /

# install agent
COPY --from=devimg \
    /go/bin/vpp-agent \
    /go/bin/vpp-agent-ctl \
 /bin/

# copy configs
COPY \
    etcd.conf \
    kafka.conf \
    govpp.conf \
    vpp-plugin.conf \
    linux-plugin.conf \
 /opt/vpp-agent/dev/

COPY vpp.conf /etc/vpp/vpp.conf
COPY supervisord.conf /etc/supervisord/supervisord.conf

# copy scripts
COPY \
    exec_agent.sh \
    supervisord_kill.py \
 /usr/bin/

WORKDIR /root/

# run supervisor as the default executable
CMD rm -f /dev/shm/db /dev/shm/global_vm /dev/shm/vpe-api && \
    /usr/bin/supervisord -c /etc/supervisord/supervisord.conf
