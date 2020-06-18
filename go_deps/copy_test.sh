#!/bin/sh
set -e

SRC="${2}"
BASE="$(basename $SRC)"
EXT=".test"
DEST="$1/${BASE%${EXT}}-${RANDOM}${EXT}"
cp "$2" "${DEST}"
# Important! This output must be kept in sync with the regex in the go_build
# task driver.
echo "Wrote test executable ${DEST}"
