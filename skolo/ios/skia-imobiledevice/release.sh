#!/bin/bash
# Upload built Debian package to CIPD for deployment a la
# http://go/skia-ansible-binaries.
#
# At the moment, this supports only a single architecture, though we follow the
# usual skia-ansible-binaries directory layout for consistency. Pay the cost of
# cross-compilation if and when we ever need multiple architectures.

set -x -e

if [ "$#" -ne 2 ]
then
    echo "Usage: ./release.sh <path to imobiledevice.deb> <username>"
    exit 1
fi

DEB="$1"
# We take an explicit username so the common case of building and releasing from
# a rpi doesn't result in "chrome-bot" as the username in the version string:
USER="$2"

VERSION=`../bash/release_tag.sh`
DEB_DIR=build/Linux/aarch64

mkdir -p "${DEB_DIR}"
cp "${DEB}" "${DEB_DIR}/skia-imobiledevice.deb"
cipd create -pkg-def=cipd.yml --tag version:${VERSION}
../../../bash/ansible-release.sh imobiledevice ${VERSION}
