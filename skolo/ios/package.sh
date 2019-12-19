#!/bin/bash

set -x -e

APPNAME=imobiledevice
SERVICE_FILE="path-to-service-file.service"

# Builds and uploads a debian package for skiacorrectness.
SYSTEMD="usbmuxd.service"
DESCRIPTION="Latest versions of libimobiledevice and related tools."
IN_DIR="$(pwd)/out"

# Make sure we restart udev rules and bypass the upload.
UDEV_LIB_RELOAD=True
BYPASS_UPLOAD=True
DEPENDS="libzip2"

# Copy files into the right locations in ${ROOT}.
copy_release_files()
{
INSTALL="fakeroot install -D --verbose --backup=none --group=root --owner=root"
INSTALL_DIR="fakeroot install -d --verbose --backup=none --group=root --owner=root"

cp -r ${IN_DIR}/sbin  ${ROOT}/sbin
cp -r ${IN_DIR}/bin   ${ROOT}/bin
cp -r ${IN_DIR}/lib   ${ROOT}/lib
cp -r ${IN_DIR}/share ${ROOT}/share

${INSTALL} --mode=644 -T ${IN_DIR}/udev-rules/39-usbmuxd.rules ${ROOT}/etc/udev/rules.d/39-usbmuxd.rules
${INSTALL} --mode=644 -T ${IN_DIR}/systemd/usbmuxd.service     ${ROOT}/etc/systemd/system/usbmuxd.service
}

source ../../bash/release.sh
