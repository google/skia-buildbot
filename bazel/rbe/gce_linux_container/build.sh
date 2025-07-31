#!/bin/bash

set -x -e

APPNAME=infra-rbe-linux

# Copy files into the right locations in ${ROOT}.
copy_release_files()
{
INSTALL="install -D --verbose --backup=none"
INSTALL_DIR="install -d --verbose --backup=none"
${INSTALL} --mode=644 -T Dockerfile ${ROOT}/Dockerfile
}

source ../../../bash/docker_build.sh
