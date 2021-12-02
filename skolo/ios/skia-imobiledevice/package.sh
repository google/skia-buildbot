#!/bin/bash
# Build a Debian package for libimobiledevice and associated tools.

set -x -e

APPNAME=imobiledevice

SYSTEMD="usbmuxd.service"
DESCRIPTION="Patched versions of libimobiledevice and related tools"
IN_DIR="$(pwd)/out"
OUT_DIR=""

# Make sure we restart udev rules and bypass the upload.
UDEV_LIB_RELOAD=True
BYPASS_UPLOAD=True

# DEPENDS, BREAKS, and CONFLICTS are unions of those fields across all Debian
# packages in the libimobiledevice family:
DEPENDS="fuse, libc6 (>= 2.17), libfuse2 (>= 2.8), libzip4 (>= 0.10), libssl1.1 (>= 1.1.0), libusb-1.0-0 (>= 2:1.0.22), adduser"
BREAKS="usbmuxd (<< 1.1.1~git20181007.f838cf6-1)"
CONFLICTS="libplist3, libusbmuxd6, usbmuxd, ideviceinstaller, ifuse, libimobiledevice6"

VERSION=1.1

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

source ../../../bash/release.sh
