#!/bin/bash
set -e -x

echo $1
TARGET=$1
PORT=$2

ssh -i /home/jcgregorio/.ssh/id_ed25519 ${TARGET} -p ${PORT} mkdir -p /tmp/bazel/
rsync --copy-links switchboard/*  -e "ssh -p ${PORT}" ${TARGET}:/tmp/bazel/
ssh -i /home/jcgregorio/.ssh/id_ed25519 ${TARGET} -p ${PORT} /tmp/bazel/hello_over_adb.sh