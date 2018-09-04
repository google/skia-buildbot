#!/bin/bash

set -x -e

APPNAME=imobiledevice
SERVICE_FILE="path-to-service-file.service"

# Builds and uploads a debian package for skiacorrectness.
SYSTEMD="usbmuxd.service"
DESCRIPTION="Latest versions of libimobiledevice and related tools."
IN_DIR="$(pwd)/out"
OUT_DIR="usr/local"
ASSET_DIR="assets"

# Make sure we restart udev rules and bypass the upload.
UDEV_LIB_RELOAD=True
BYPASS_UPLOAD=True
DEPENDS="libzip2"

# Fix the paths in the config files.
sed -i "s+${IN_DIR}/sbin+/usr/local/sbin+g" ${IN_DIR}/systemd/usbmuxd.service
sed -i "s+${IN_DIR}/var/run+/var/run+g"     ${IN_DIR}/systemd/usbmuxd.service
sed -i "s+${IN_DIR}/sbin+/usr/local/sbin+g" ${IN_DIR}/udev-rules/*.rules

# Copy files into the right locations in ${ROOT}.
copy_release_files()
{
INSTALL="sudo install -D --verbose --backup=none --group=root --owner=root"
INSTALL_DIR="sudo install -d --verbose --backup=none --group=root --owner=root"

${INSTALL_DIR} --mode=755                                             ${ROOT}/${OUT_DIR}/sbin
${INSTALL}     --mode=755 -T ${IN_DIR}/sbin/usbmuxd                   ${ROOT}/${OUT_DIR}/sbin/usbmuxd

${INSTALL_DIR} --mode=755                                             ${ROOT}/${OUT_DIR}/bin
${INSTALL}     --mode=755 -T ${IN_DIR}/bin/idevicename                ${ROOT}/${OUT_DIR}/bin/idevicename
${INSTALL}     --mode=755 -T ${IN_DIR}/bin/ifuse                      ${ROOT}/${OUT_DIR}/bin/ifuse
${INSTALL}     --mode=755 -T ${IN_DIR}/bin/ideviceprovision           ${ROOT}/${OUT_DIR}/bin/ideviceprovision
${INSTALL}     --mode=755 -T ${IN_DIR}/bin/ideviceinstaller           ${ROOT}/${OUT_DIR}/bin/ideviceinstaller
${INSTALL}     --mode=755 -T ${IN_DIR}/bin/idevicesyslog              ${ROOT}/${OUT_DIR}/bin/idevicesyslog
${INSTALL}     --mode=755 -T ${IN_DIR}/bin/idevicebackup              ${ROOT}/${OUT_DIR}/bin/idevicebackup
${INSTALL}     --mode=755 -T ${IN_DIR}/bin/plistutil                  ${ROOT}/${OUT_DIR}/bin/plistutil
${INSTALL}     --mode=755 -T ${IN_DIR}/bin/idevicebackup2             ${ROOT}/${OUT_DIR}/bin/idevicebackup2
${INSTALL}     --mode=755 -T ${IN_DIR}/bin/idevicepair                ${ROOT}/${OUT_DIR}/bin/idevicepair
${INSTALL}     --mode=755 -T ${IN_DIR}/bin/ideviceimagemounter        ${ROOT}/${OUT_DIR}/bin/ideviceimagemounter
${INSTALL}     --mode=755 -T ${IN_DIR}/bin/idevicedebugserverproxy    ${ROOT}/${OUT_DIR}/bin/idevicedebugserverproxy
${INSTALL}     --mode=755 -T ${IN_DIR}/bin/idevicenotificationproxy   ${ROOT}/${OUT_DIR}/bin/idevicenotificationproxy
${INSTALL}     --mode=755 -T ${IN_DIR}/bin/idevice_id                 ${ROOT}/${OUT_DIR}/bin/idevice_id
${INSTALL}     --mode=755 -T ${IN_DIR}/bin/idevicedebug               ${ROOT}/${OUT_DIR}/bin/idevicedebug
${INSTALL}     --mode=755 -T ${IN_DIR}/bin/idevicediagnostics         ${ROOT}/${OUT_DIR}/bin/idevicediagnostics
${INSTALL}     --mode=755 -T ${IN_DIR}/bin/idevicedate                ${ROOT}/${OUT_DIR}/bin/idevicedate
${INSTALL}     --mode=755 -T ${IN_DIR}/bin/ideviceinfo                ${ROOT}/${OUT_DIR}/bin/ideviceinfo
${INSTALL}     --mode=755 -T ${IN_DIR}/bin/idevicecrashreport         ${ROOT}/${OUT_DIR}/bin/idevicecrashreport
${INSTALL}     --mode=755 -T ${IN_DIR}/bin/idevicescreenshot          ${ROOT}/${OUT_DIR}/bin/idevicescreenshot
${INSTALL}     --mode=755 -T ${IN_DIR}/bin/ideviceenterrecovery       ${ROOT}/${OUT_DIR}/bin/ideviceenterrecovery

${INSTALL_DIR} --mode=755                                             ${ROOT}/${OUT_DIR}/lib
${INSTALL}     --mode=644 -T ${IN_DIR}/lib/libplist++.a               ${ROOT}/${OUT_DIR}/lib/libplist++.a
${INSTALL}     --mode=644 -T ${IN_DIR}/lib/libplist++.so              ${ROOT}/${OUT_DIR}/lib/libplist++.so
${INSTALL}     --mode=644 -T ${IN_DIR}/lib/libplist++.so.3            ${ROOT}/${OUT_DIR}/lib/libplist++.so.3
${INSTALL}     --mode=644 -T ${IN_DIR}/lib/libplist++.so.3.1.0        ${ROOT}/${OUT_DIR}/lib/libplist++.so.3.0.0
${INSTALL}     --mode=644 -T ${IN_DIR}/lib/libplist.a                 ${ROOT}/${OUT_DIR}/lib/libplist.a
${INSTALL}     --mode=644 -T ${IN_DIR}/lib/libplist.so                ${ROOT}/${OUT_DIR}/lib/libplist.so
${INSTALL}     --mode=644 -T ${IN_DIR}/lib/libplist.so.3              ${ROOT}/${OUT_DIR}/lib/libplist.so.3
${INSTALL}     --mode=644 -T ${IN_DIR}/lib/libplist.so.3.1.0          ${ROOT}/${OUT_DIR}/lib/libplist.so.3.0.0
${INSTALL}     --mode=644 -T ${IN_DIR}/lib/libusbmuxd.so.4            ${ROOT}/${OUT_DIR}/lib/libusbmuxd.so.4
${INSTALL}     --mode=644 -T ${IN_DIR}/lib/libusbmuxd.a               ${ROOT}/${OUT_DIR}/lib/libusbmuxd.a
${INSTALL}     --mode=644 -T ${IN_DIR}/lib/libusbmuxd.so              ${ROOT}/${OUT_DIR}/lib/libusbmuxd.so
${INSTALL}     --mode=644 -T ${IN_DIR}/lib/libusbmuxd.so.4.0.0        ${ROOT}/${OUT_DIR}/lib/libusbmuxd.so.4.0.0
${INSTALL}     --mode=644 -T ${IN_DIR}/lib/libimobiledevice.a         ${ROOT}/${OUT_DIR}/lib/libimobiledevice.a
${INSTALL}     --mode=644 -T ${IN_DIR}/lib/libimobiledevice.so        ${ROOT}/${OUT_DIR}/lib/libimobiledevice.so
${INSTALL}     --mode=644 -T ${IN_DIR}/lib/libimobiledevice.so.6      ${ROOT}/${OUT_DIR}/lib/libimobiledevice.so.6
${INSTALL}     --mode=644 -T ${IN_DIR}/lib/libimobiledevice.so.6.0.0  ${ROOT}/${OUT_DIR}/lib/libimobiledevice.so.6.0.0

${INSTALL}     --mode=644 -T ${IN_DIR}/udev-rules/39-usbmuxd.rules    ${ROOT}/etc/udev/rules.d/39-usbmuxd.rules
${INSTALL}     --mode=644 -T ${IN_DIR}/systemd/usbmuxd.service        ${ROOT}/etc/systemd/system/usbmuxd.service

# Recursively install the assets.
find ${ASSET_DIR} -type d -exec ${INSTALL_DIR} --mode=755 ${ROOT}/usr/local/share/{} \;
find ${ASSET_DIR} -type f -exec ${INSTALL} --mode=644 -T {} ${ROOT}/usr/local/share/{} \;
}

source ../../bash/release.sh
