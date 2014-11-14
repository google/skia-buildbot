#!/bin/bash
#
# Creates persistent disks for the specified chromecompute instances.
#
# Copyright 2014 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)

source vm_config.sh

for MACHINE_IP in $(seq $VM_BOT_COUNT_START $VM_BOT_COUNT_END); do
  DISK_NAMES="$DISK_NAMES skia-disk"-`printf "%03d" ${MACHINE_IP}`
done

$GCOMPUTE_CMD adddisk $DISK_NAMES --size_gb=$PERSISTENT_DISK_SIZE_GB --zone=$ZONE
