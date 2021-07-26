#!/bin/bash

set -x

# Record the directory of this file.
REL=$(dirname "$0")

# Check argument count is valid.
if [ $# -ne 2 ]; then
    exit 1
fi

HOSTNAME=$1;
MOUNT=$2

echo ${HOSTNAME} > ${MOUNT}/etc/hostname
install -D --mode=600 ${REL}/../../authorized_keys ${MOUNT}/root/.ssh/authorized_keys
sync --file-system ${MOUNT}/root/.ssh/authorized_keys
