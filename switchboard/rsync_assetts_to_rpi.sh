#!/bin/bash
set -e -x

TARGET=$1

ssh -i /home/jcgregorio/.ssh/id_ed25519 ${TARGET} mkdir -p /tmp/bazel/
#rsync --copy-links switchboard/* ${TARGET}:/tmp/bazel/
scp -i /home/jcgregorio/.ssh/id_ed25519 switchboard/hello ${TARGET}:/tmp/bazel/hello
scp -i /home/jcgregorio/.ssh/id_ed25519 switchboard/hello_over_adb.sh ${TARGET}:/tmp/bazel/hello_over_adb.sh
ssh -i /home/jcgregorio/.ssh/id_ed25519 ${TARGET} /tmp/bazel/hello_over_adb.sh