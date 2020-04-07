#!/bin/bash

set -x -e

# Create and upload a container image that contains the gcloud firebase emulator.
APPNAME=firestore-emulator

IMAGE=$(dirname "$0")

# Copy files into the right locations in ${ROOT}.
copy_release_files()
{
INSTALL="install -D --verbose --backup=none"

# Add the dockerfile and binary.
${INSTALL} --mode=644 -T ${IMAGE}/Dockerfile          ${ROOT}/Dockerfile
}

source ../bash/docker_build.sh