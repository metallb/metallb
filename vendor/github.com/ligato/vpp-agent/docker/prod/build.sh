#!/bin/bash
# Before run of this script you can set environmental variables
# IMAGE_TAG ... then  export them
# and to use defined values instead of default ones

cd "$(dirname "$0")"

set -e

IMAGE_TAG=${IMAGE_TAG:-'prod_vpp_agent'}

BUILDARCH=`uname -m`
case "$BUILDARCH" in
  "aarch64" )
    ;;

  "x86_64" )
    ;;
  * )
    echo "Architecture ${BUILDARCH} is not supported."
    exit
    ;;
esac

docker build  ${DOCKER_BUILD_ARGS} --tag ${IMAGE_TAG} .
