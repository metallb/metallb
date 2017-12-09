#!/bin/bash

GOBGP_PATH=${GOPATH}/src/github.com/osrg/gobgp

cd ${GOBGP_PATH}/gobgp/lib
go build -buildmode=c-shared -o libgobgp.so *.go
cd ${GOBGP_PATH}/tools/grpc/cpp
ln -s ${GOBGP_PATH}/gobgp/lib/libgobgp.h
ln -s ${GOBGP_PATH}/gobgp/lib/libgobgp.so
ln -s ${GOBGP_PATH}/api/gobgp.proto gobgp_api_client.proto
make
