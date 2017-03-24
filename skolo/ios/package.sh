#!/bin/bash

set -x -e

APPNAME=skia-libimobiledevice
SERVICE_FILE="path-to-service-file.service"

# Builds and uploads a debian package for skiacorrectness.
SYSTEMD="${APPNAME}.service"
DESCRIPTION="Latest versions of libimobiledevice and related tools."

# Copy files into the right locations in ${ROOT}.
copy_release_files()
{
INSTALL="sudo install -D --verbose --backup=none --group=root --owner=root"
INSTALL_DIR="sudo install -d --verbose --backup=none --group=root --owner=root"

${INSTALL}     --mode=755 -T ${GOPATH}/bin/correctness_migratedb ${ROOT}/usr/local/bin/correctness_migratedb
${INSTALL_DIR} --mode=755                                        ${ROOT}/usr/local/share/skiacorrectness/frontend/res/img
${INSTALL}     --mode=644 ./frontend/res/img/favicon.ico         ${ROOT}/usr/local/share/skiacorrectness/frontend/res/img/favicon.ico

${INSTALL_DIR} --mode=755                                        ${ROOT}/usr/local/share/skiacorrectness/frontend/res/js
${INSTALL}     --mode=644 ./frontend/res/js/core.js              ${ROOT}/usr/local/share/skiacorrectness/frontend/res/js/core.js

${INSTALL_DIR} --mode=755                                        ${ROOT}/usr/local/share/skiacorrectness/frontend/res/vul
${INSTALL}     --mode=644 ./frontend/res/vul/elements.html       ${ROOT}/usr/local/share/skiacorrectness/frontend/res/vul/elements.html
${INSTALL}     --mode=644 ./frontend/index.html                  ${ROOT}/usr/local/share/skiacorrectness/frontend/index.html
${INSTALL}     --mode=644 -T $SERVICE_FILE                       ${ROOT}/etc/systemd/system/${APPNAME}.service
}

source ../../bash/release.sh
done
