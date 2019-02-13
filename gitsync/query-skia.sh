#!/bin/bash

set -x

rm -f ./btgit
go build -o ./btgit ./cmd/btgit/...

./btgit \
  --bt-instance production \
  --bt-table git-repos \
  --logtostderr \
  --project skia-public \
  --repo-url https://skia.googlesource.com/skia.git \
  --limit 0 \
  --branch master

echo "Ret code ${?}"

