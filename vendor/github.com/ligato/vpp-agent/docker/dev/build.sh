#!/bin/bash
# Before run of this script you can set environmental variables
# IMAGE_TAG, DOCKERFILE, BASE_IMG, GOLANG_OS_ARCH, .. then  export them
# and to use defined values instead of default ones

cd "$(dirname "$0")"

set -e

IMAGE_TAG=${IMAGE_TAG:-'dev_vpp_agent'}
DOCKERFILE=${DOCKERFILE:-'Dockerfile'}

BASE_IMG=${BASE_IMG:-'ubuntu:18.04'}

BUILDARCH=`uname -m`
case "$BUILDARCH" in
  "aarch64" )
    GOLANG_OS_ARCH=${GOLANG_OS_ARCH:-'linux-arm64'}
    PROTOC_OS_ARCH=${PROTOC_OS_ARCH:-'linux-aarch_64'}
    ;;

  "x86_64" )
    # for AMD64 platform is used the default image (without suffix -amd64)
    GOLANG_OS_ARCH=${GOLANG_OS_ARCH:-'linux-amd64'}
    PROTOC_OS_ARCH=${PROTOC_OS_ARCH:-'linux-x86_64'}
    ;;
  * )
    echo "Architecture ${BUILDARCH} is not supported."
    exit
    ;;
esac


source ../../vpp.env
VPP_DEBUG_DEB=${VPP_DEBUG_DEB:-}

VERSION=$(git describe --always --tags --dirty)
COMMIT=$(git rev-parse HEAD)
DATE=$(git log -1 --format="%ct" | xargs -I{} date -d @{} +'%Y-%m-%dT%H:%M%:z')

echo "=============================="
echo
echo "VPP"
echo "-----------------------------"
echo " repo URL: ${VPP_REPO_URL}"
echo " commit:   ${VPP_COMMIT}"
echo "-----------------------------"
echo
echo "Agent"
echo "-----------------------------"
echo " version: ${VERSION}"
echo " commit:  ${COMMIT}"
echo " date:    ${DATE}"
echo "-----------------------------"
echo
echo "base image: ${BASE_IMG}"
echo "image tag:  ${IMAGE_TAG}"
echo "architecture: ${BUILDARCH}"
echo "=============================="

docker build -f ${DOCKERFILE} \
    --tag ${IMAGE_TAG} \
    --build-arg BASE_IMG=${BASE_IMG} \
    --build-arg VPP_COMMIT=${VPP_COMMIT} \
    --build-arg VPP_REPO_URL=${VPP_REPO_URL} \
    --build-arg VPP_DEBUG_DEB=${VPP_DEBUG_DEB} \
    --build-arg GOLANG_OS_ARCH=${GOLANG_OS_ARCH} \
    --build-arg PROTOC_OS_ARCH=${PROTOC_OS_ARCH} \
    --build-arg VERSION=${VERSION} \
    --build-arg COMMIT=${COMMIT} \
    --build-arg DATE=${DATE} \
    ${DOCKER_BUILD_ARGS} ../..
