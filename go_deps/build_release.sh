#!/bin/bash

set -x -e

# Create and upload a container image for go_deps
APPNAME=go_deps

# Copy files into the right locations in ${ROOT}.
copy_release_files()
{
INSTALL="install -D --verbose --backup=none"
INSTALL_DIR="install -d --verbose --backup=none"

# Add the dockerfile and binary.
${INSTALL} --mode=444 -T ../go.mod     ${ROOT}/go.mod
${INSTALL} --mode=444 -T ../go.sum     ${ROOT}/go.sum
${INSTALL} --mode=644 -T ./Dockerfile  ${ROOT}/Dockerfile
for script in $(ls install*.sh); do
  ${INSTALL} --mode=755 -T ./${script} ${ROOT}/${script}
done
}

source ../bash/docker_build.sh
docker inspect --format='{{index .RepoDigests 0}}' go_deps > image.sha256