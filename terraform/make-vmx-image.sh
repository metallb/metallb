#!/bin/bash

set -e

if [[ $# -ne 1 ]]; then
    echo "Usage: $0 project-name"
    exit 1
fi

gcloud compute images delete --quiet --project=$1 debian-vmx || true
gcloud compute images create debian-vmx --project=$1 --source-image-family debian-9 --source-image-project debian-cloud --licenses "https://www.googleapis.com/compute/v1/projects/vm-options/global/licenses/enable-vmx"
