#!/bin/bash
# Usage: examples
#    ./push_image.sh
#    BRANCH_HEAD_TAG='git describe' ./push_image.sh
#    REPO_OWNER=stanislavchlebec BRANCH_HEAD_TAG=`git describe` ./push_image.sh
#    LOCAL_IMAGE='prod_vpp_agent:latest' IMAGE_NAME='vpp-agent' ./push_image.sh

# Warning: use only IMMEDIATELY after docker/dev/build.sh to prevent INCONSISTENCIES such as 
#          a) after building image you switch to other branch which will result in mismatch of version of image and its tag
#          b) you do not build the new image but only simply run this script which will result in mismatch version of image and its tag because the image is older than repository 
LOCAL_IMAGE='prod_vpp_agent:latest' IMAGE_NAME='vpp-agent' ../dev/push_image.sh
