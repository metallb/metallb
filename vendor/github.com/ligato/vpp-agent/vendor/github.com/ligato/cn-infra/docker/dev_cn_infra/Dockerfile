FROM ubuntu:16.04

ARG AGENT_COMMIT="xxx"

RUN apt-get update && apt-get install -y \
    sudo wget git build-essential gdb vim nano python \
    iproute2 iputils-ping inetutils-traceroute libapr1 supervisor telnet netcat

RUN mkdir /opt/agent
RUN mkdir /opt/agent/dev

WORKDIR /opt/agent/dev


# install Go & Glide
RUN wget https://storage.googleapis.com/golang/go1.9.2.linux-amd64.tar.gz && \
    tar -xvf go1.9.2.linux-amd64.tar.gz && \
    mv go /usr/local && \
    rm -f go1.9.2.linux-amd64.tar.gz

# build & install Protobuf & gogo protobuf compiler
RUN apt-get install -y make && \
    apt-get install -y autoconf automake libtool curl make g++ unzip

RUN git clone https://github.com/google/protobuf.git && \
    cd protobuf && ./autogen.sh && ./configure && \
    make -j4 && \
    make install && \
    ldconfig && \
    cd .. && rm -rf protobuf

RUN apt-get install -y golang-gogoprotobuf-dev

# copy and execute agent build script
COPY build-agent.sh .
RUN ./build-agent.sh ${AGENT_COMMIT}

COPY etcd.conf .
COPY kafka.conf .

WORKDIR /root/

# add supervisor conf file
COPY supervisord.conf /etc/supervisord.conf

# run supervisor as the default executable
CMD ["/usr/bin/supervisord", "-c", "/etc/supervisord.conf"]
