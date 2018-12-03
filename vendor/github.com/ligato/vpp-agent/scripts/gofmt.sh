#!/bin/bash

find $(pwd) -mount -name "*.go" -type f \
    -not -path $(pwd)"/vendor/*" \
    -not -name "pkgreflect.go" \
    -not -name "bindata.go" \
    -exec gofmt -w -s {} +
