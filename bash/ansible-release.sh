#!/bin/bash

# Tags the version of the application to be used by Ansible playbooks.
#
# See http://go/skia-ansible-binaries

set -e
set -x
set -o pipefail

if [[ $# -ne 2 ]]; then
    echo "$0 <application> <version>"
    exit 1
fi

APPNAME=$1
VERSION=$2

# Create temp dir and cd into it.
cd "$(mktemp -d)"

# Clone k8s-config.
git clone https://skia.googlesource.com/k8s-config

cd k8s-config

# Create a branch.
git new-branch update-version

# Write the tag file.
mkdir -p "./ansible-tags/$APPNAME"
echo "$VERSION" > "./ansible-tags/$APPNAME/version.txt"

# Commit via rubberstamper.
git add "./ansible-tags/$APPNAME/version.txt"
git commit -m "Update Ansible version of $APPNAME to $VERSION."

git cl upload \
    --skip-title \
    --reviewers="rubber-stamper@appspot.gserviceaccount.com" \
    --enable-auto-submit \
    --send-mail \
    --force
