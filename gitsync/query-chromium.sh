#!/bin/bash

set -x

rm -f ./btgit
go build -o ./btgit ./cmd/btgit/...

./btgit \
  --bt-instance production \
  --bt-table git-repos \
  --logtostderr \
  --project skia-public \
  --repo-url https://chromium.googlesource.com/chromium/src \
  --limit 1000 \
  --branch=master

echo "Ret code ${?}"

