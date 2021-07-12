#!/bin/bash

# Record the directory of this file.
REL=$(dirname "$0")

# Check argument count is valid.
if [ $# -ne 2 ]; then
    exit 1
fi

HOSTNAME=$1;
MOUNT=$2

echo ${HOSTNAME} > ${MOUNT}/etc/hostname
install -D --verbose --mode=666              ${REL}/../../authorized_keys ${MOUNT}/opt/skolo/authorized_keys
install -D --verbose --owner=root --mode=755 ${REL}/setup.sh              ${MOUNT}/usr/local/sbin/rpi-set-sysconf
