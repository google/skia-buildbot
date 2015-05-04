#!/bin/sh
# This bash script checks if the md5sum of ${GOPATH}/pdfium_test matches
# the one in pdfium.md5. If not it will download it from gloud storage.

set -x -e

OLD_DIR=`pwd`

cd "$( dirname "${BASH_SOURCE[0]}" )"
source ./build_setup.sh

EXE_PATH="${GOPATH}/bin/${EXECUTABLE}"
EXE_MD5=""

# If there is no hash of the executable, we have nothing to do.
if [[ ! -f "$MD5_FILE" ]]; then
    echo "File ${MD5_FILE} doesn't exist."
    exit 1
fi
CURR_MD5="$(<${MD5_FILE})"
CLOUD_PATH="${CLOUD_PATH_BASE}-${CURR_MD5}"

# Get the MD5 of the executable if it exists.
if [[ -f "${EXE_PATH}" ]]; then
    EXE_MD5=`md5sum ${EXE_PATH} | awk '{ print $1 }'`
fi

# If the MD5s do not match then download the right version.
if [[ "$CURR_MD5" != "$EXE_MD5" ]]; then
    gsutil cp "${CLOUD_PATH}" "${EXE_PATH}"
    chmod 755 "$EXE_PATH"
else
    echo "${EXECUTABLE} up to date. Nothing to do."
fi

cd "$OLD_DIR"
