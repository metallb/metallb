#!/bin/bash

set -e

# setup Go paths
export GOROOT=/usr/local/go
export GOPATH=$HOME/go
export PATH=$PATH:$GOROOT/bin:$GOPATH/bin
echo "export GOROOT=$GOROOT" >> ~/.bashrc
echo "export GOPATH=$GOPATH" >> ~/.bashrc
echo "export PATH=$PATH" >> ~/.bashrc
mkdir -p $GOPATH/{bin,pkg,src}

# install golint, gvt & Glide
#go get -u github.com/golang/lint/golint
#go get -u github.com/FiloSottile/gvt
#curl https://glide.sh/get | sh

# checkout agent code
mkdir -p $GOPATH/src/github.com/ligato
cd $GOPATH/src/github.com/ligato
git clone https://github.com/ligato/cn-infra.git

# build the agent
cd $GOPATH/src/github.com/ligato/cn-infra
git checkout $1
make

cp examples/simple-agent/simple-agent $GOPATH/bin/
#make install
#make test
#make generate
