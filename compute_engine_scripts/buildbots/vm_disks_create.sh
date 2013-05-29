#!/bin/bash
#
# Create all GCE VM disks.
#
# Copyright 2013 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)

source vm_config.sh

for VM in $VM_NAMES; do
  $GCOMPUTE_CMD adddisk --size_gb=${DISK_SIZE} ${VM}-disk-${ZONE_TAG} \
    --zone=$ZONE
done

exit 0
