#!/bin/bash
# This script builds pdfium in place. Upon succesfull build it saves the
# MD5 sum in 'pdfium.md5' and uploads the executable to cloud storage, if it's
# not already there.
set -x -e

OLD_DIR=`pwd`

cd "$( dirname "${BASH_SOURCE[0]}" )"
source ./build_setup.sh

REPO="https://pdfium.googlesource.com/pdfium"
COMMIT=$(<${PDFIUM_COMMIT_FILE})

TARGET_DIRECTORY=`pwd`
PDFIUM_BUILD_DIRECTORY="./build"

# Get rid of the MD5 file to detect failures.
rm -f "${TARGET_DIRECTORY}/${MD5_FILE}"

mkdir -p "$PDFIUM_BUILD_DIRECTORY"
cd "$PDFIUM_BUILD_DIRECTORY"

if ! [ -d pdfium ]; then
    git clone "$REPO"
fi

cd pdfium

if ! [ -f ".gclient" ] ; then
    gclient config --name . --unmanaged "$REPO"
fi

if [ "$(git diff --shortstat)" ] ; then
    git stash save
fi

if [ "$(git rev-parse HEAD)" != "$COMMIT" ]; then
    git fetch
    git checkout "$COMMIT"
fi

if [ "$(git hash-object DEPS)" != "$(git config sync-deps.last)" ] ; then
    gclient sync
    git config sync-deps.last "$(git hash-object DEPS)"
fi

GYP_GENERATORS=ninja build/gyp_pdfium

ninja -C out/Release ${EXECUTABLE}

mkdir -p "$TARGET_DIRECTORY"

cp -a out/Release/${EXECUTABLE} "${TARGET_DIRECTORY}/${EXECUTABLE}"

# Get the MD5 hash
MD5=`md5sum ${TARGET_DIRECTORY}/${EXECUTABLE} | awk '{ print $1 }'`
CLOUD_PATH="${CLOUD_PATH_BASE}-${MD5}"

# Upload the file to GS if it's not already there.
gsutil cp -n -a public-read "${TARGET_DIRECTORY}/${EXECUTABLE}" "${CLOUD_PATH}"

# Write the local MD5 hash of the binary and remove the binary.
echo ${MD5} > ${TARGET_DIRECTORY}/${MD5_FILE}
rm "${TARGET_DIRECTORY}/${EXECUTABLE}"

cd "$OLD_DIR"
