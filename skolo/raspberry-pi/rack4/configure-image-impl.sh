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
mkdir -p                        ${MOUNT}/opt/skolo
cp ${REL}/../../authorized_keys ${MOUNT}/opt/skolo/authorized_keys
cp ${REL}/setup.sh              ${MOUNT}/opt/skolo/setup.sh
chmod +x                        ${MOUNT}/opt/skolo/setup.sh
chown root:root                 ${MOUNT}/opt/skolo/setup.sh
cp ${REL}/skolo-setup.service   ${MOUNT}/etc/systemd/system/skolo-setup.service
chmod 644                       ${MOUNT}/etc/systemd/system/skolo-setup.service
chown root:root                 ${MOUNT}/etc/systemd/system/skolo-setup.service
