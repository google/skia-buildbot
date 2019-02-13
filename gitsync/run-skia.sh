#!/bin/bash

set -x

REPO_DIR=./repo-dirs
mkdir -p ${REPO_DIR}

rm -f ./gitsync
go build -o ./gitsync ./cmd/gitsync/...

./gitsync \
  --bt-instance production \
  --bt-table git-repos \
  --data-dir ./repo-dirs \
  --logtostderr \
  --project skia-public \
  --repo-url https://skia.googlesource.com/skia.git \
  --repo-url https://chromium.googlesource.com/chromium/src \
  --repo-url https://pdfium.googlesource.com/pdfium \

echo "Ret code ${?}"

