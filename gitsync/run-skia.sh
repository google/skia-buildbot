#!/bin/bash

set -x

# TMP_DIR=`mktemp -d`

rm -f ./gitsync
go build -o ./gitsync ./go/gitsync/

./gitsync \
  --instance git-bt \
  --logtostderr \
  --project skia-public \
  --repo_dir /home/stephana/dev/skia \
  --repo_url https://skia.googlesource.com/skia.git \
  --table git-repos

echo "Ret code ${?}"

