#!/bin/bash

# Record the directory of this file.
REL=$(dirname "$0")

# Check argument count is valid.
if [ $# -ne 2 ]; then
    exit 1
fi

HOSTNAME=$1;
MOUNT=$2

echo name=${HOSTNAME} > ${MOUNT}/boot/firmware/sysconf.txt
install -D --verbose --mode=666              ${REL}/../../authorized_keys ${MOUNT}/opt/skolo/authorized_keys
install -D --verbose --owner=root --mode=755 ${REL}/setup.sh              ${MOUNT}/opt/skolo/setup.sh
install -D --verbose --owner=root --mode=644 ${REL}/skolo-setup.service   ${MOUNT}/etc/systemd/system/skolo-setup.service