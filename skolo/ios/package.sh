#!/bin/bash

set -x -e

APPNAME=imobiledevice
SERVICE_FILE="path-to-service-file.service"

# Builds and uploads a debian package for libimobiledevice.
SYSTEMD="usbmuxd.service"
DESCRIPTION="Latest versions of libimobiledevice and related tools."
IN_DIR="$(pwd)/out"
OUT_DIR=""

# Make sure we restart udev rules and bypass the upload.
UDEV_LIB_RELOAD=True
BYPASS_UPLOAD=True
DEPENDS="libzip2"

# Fix the paths in the config files.
sed -i "s+${IN_DIR}/sbin+${OUT_DIR}/sbin+g" ${IN_DIR}/systemd/usbmuxd.service
sed -i "s+${IN_DIR}/var/run+/var/run+g"     ${IN_DIR}/systemd/usbmuxd.service
sed -i "s+${IN_DIR}/sbin+${OUT_DIR}/sbin+g" ${IN_DIR}/udev-rules/*.rules

# Copy files into the right locations in ${ROOT}.
copy_release_files()
{
INSTALL="fakeroot install -D --verbose --backup=none --group=root --owner=root"
INSTALL_DIR="fakeroot install -d --verbose --backup=none --group=root --owner=root"

${INSTALL_DIR} ${ROOT}${OUT_DIR}/
cp -r ${IN_DIR}/sbin      ${ROOT}${OUT_DIR}/sbin
cp -r ${IN_DIR}/bin       ${ROOT}${OUT_DIR}/bin
cp -r ${IN_DIR}/lib       ${ROOT}${OUT_DIR}/lib
cp -r ${IN_DIR}/share     ${ROOT}${OUT_DIR}/share

${INSTALL} --mode=644 -T ${IN_DIR}/udev-rules/39-usbmuxd.rules ${ROOT}/etc/udev/rules.d/39-usbmuxd.rules
${INSTALL} --mode=644 -T ${IN_DIR}/systemd/usbmuxd.service     ${ROOT}/etc/systemd/system/usbmuxd.service
}

source ../../bash/release.sh
