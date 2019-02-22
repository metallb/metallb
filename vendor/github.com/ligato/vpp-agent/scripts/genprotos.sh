#!/usr/bin/env bash

set -euo pipefail

API_DIR=${1:-$PWD/api}
cd api

protos=$(find . -type f -name '*.proto')

for proto in $protos; do
	echo " - $proto";
	protoc \
		--proto_path=${API_DIR} \
		--proto_path=. \
		--proto_path=$GOPATH/src/github.com/gogo/protobuf \
		--proto_path=$GOPATH/src \
		--gogo_out=plugins=grpc,\
Mgoogle/protobuf/any.proto=github.com/gogo/protobuf/types:$GOPATH/src \
		"${API_DIR}/$proto";
done
