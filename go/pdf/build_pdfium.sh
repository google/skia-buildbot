#!/bin/sh

if [ -z "$1" ] || [ -z "$2" ] ; then
    echo 'usage: build_pdfium.sh TARGET_DIRECTORY PDFIUM_BUILD_DIRECTORY' >&2
    exit 1
fi

set -x -e

REPO="https://pdfium.googlesource.com/pdfium"
COMMIT="1ddf056da74de0a34631b8a719f4f02b4ec82144"

TARGET_DIRECTORY="$1"
PDFIUM_BUILD_DIRECTORY="$2"

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

ninja -C out/Release pdfium_test

mkdir -p "$TARGET_DIRECTORY"

cp -a out/Release/pdfium_test "${TARGET_DIRECTORY}/pdfium_test"
