#!/bin/bash

set -e

corpus=$(find fuzz-data/corpus -name 'autogen-*' | wc -l)

gen_fuzz() {
    echo "Generating initial fuzzing corpus"
    go run fuzz-gen.go -workdir=fuzz-data/corpus -num=100000
}    

fuzz() {
    echo "Building fuzz binary"
    go-fuzz-build go.universe.tf/metallb/internal/bgp/message
    echo "Fuzzing..."
    go-fuzz -bin=./message-fuzz.zip -workdir=fuzz-data
}

case $1 in
    "")
        if [[ $corpus == 0 ]]; then
            gen_fuzz
        fi
        fuzz
        ;;
    force)
        gen_fuzz
        fuzz
        ;;
    clean)
        find fuzz-data/corpus -name 'autogen-*' -delete
esac
