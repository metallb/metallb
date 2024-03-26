#!/bin/bash -e
set -x

if [[ ! -v GITHUB_TOKEN ]]; then
    echo "GITHUB_TOKEN is not set, please set it with a token with read permissions on commits and PRs"
    exit 1
fi

script_dir=$(dirname "$(readlink -f "$0")")

branch=$1
from=$2
to=$3
release_notes=$(mktemp)

end() {
    rm $release_notes
}

trap end EXIT SIGINT SIGTERM SIGSTOP

GOFLAGS=-mod=mod go run k8s.io/release/cmd/release-notes@v0.16.5 \
    --branch $branch \
    --required-author "" \
    --org metallb \
    --dependencies=false \
    --repo metallb \
    --start-sha $from \
    --end-sha $to \
    --output $release_notes

cat $release_notes


echo "Contributors"
git log --format="%aN" $(git merge-base $to $from)..$to | sort -u | tr '\n' ',' | sed -e 's/,/, /g'
