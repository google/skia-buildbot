#!/bin/bash

set -ex

# Record the directory of this file.
REL=$(dirname "$0")

# Check argument count is valid.
if [ $# -ne 2 ]; then
    exit 1
fi

HOSTNAME=$1;
MOUNT=$2

echo ${HOSTNAME} > ${MOUNT}/etc/hostname
if [ "$(uname -s)" = Darwin ]; then
    # The Mac version of install acts a bit differently.
    mkdir -p ${MOUNT}/root/.ssh
    install -m 600 ${REL}/../authorized_keys ${MOUNT}/root/.ssh/authorized_keys
else
    install -D --mode=600 ${REL}/../authorized_keys ${MOUNT}/root/.ssh/authorized_keys
fi
sync --file-system ${MOUNT}/root/.ssh/authorized_keys
