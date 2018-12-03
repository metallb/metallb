#!/bin/bash

find $(pwd) -mount -name "*.go" -type f -not -path $(pwd)"/vendor/*" -exec gofmt -w -s {} +
