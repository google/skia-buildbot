#!/bin/bash
if [ -z "$CLUSTER" ]; then
  echo "This script should not be run directly."
  exit 1
fi
APPNAME=autoroll-fe${SUFFIX}

set -x -e

# Copy files into the right locations in ${ROOT}.
copy_release_files()
{
INSTALL="install -D --verbose --backup=none"
INSTALL_DIR="install -d --verbose --backup=none"
${INSTALL} --mode=644 -T ./go/autoroll-fe/Dockerfile ${ROOT}/Dockerfile
${INSTALL} --mode=755 -T ${GOPATH}/bin/autoroll-fe   ${ROOT}/usr/local/bin/autoroll-fe
${INSTALL_DIR}                                       ${ROOT}/usr/local/share/autoroll-fe
cp -r                    ./dist                      ${ROOT}/usr/local/share/autoroll-fe/dist
chmod -R                 777                         ${ROOT}/usr/local/share/autoroll-fe/dist
${INSTALL_DIR}                                       ${ROOT}/usr/local/share/autoroll-fe/configs
# TODO(borenet): Use a non-temp directory for configs.
cp /tmp/skia-autoroll-internal-config/${CLUSTER}/* ${ROOT}/usr/local/share/autoroll-fe/configs
chmod -R                 777                       ${ROOT}/usr/local/share/autoroll-fe/configs
}

source ../bash/docker_build.sh
