#!/usr/bin/env bash

bin_dir=$1/bin/

mkdir -p ${bin_dir}
GOBIN=${bin_dir} go install sigs.k8s.io/controller-runtime/tools/setup-envtest@c7e1dc9
