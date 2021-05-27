#!/bin/bash
set -e -x

TARGET=$1

ssh -i /home/jcgregorio/.ssh/id_ed25519 ${TARGET} mkdir -p /tmp/bazel/
rsync --copy-links switchboard/* ${TARGET}:/tmp/bazel/
ssh -i /home/jcgregorio/.ssh/id_ed25519 ${TARGET} /tmp/bazel/hello_over_adb.sh