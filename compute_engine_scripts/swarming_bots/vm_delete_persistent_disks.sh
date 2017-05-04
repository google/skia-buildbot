#!/bin/bash
#
# Creates persistent disks for the specified Skia GCE instances.
#
# Copyright 2014 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)

source vm_config.sh

for MACHINE_IP in $(seq $VM_BOT_COUNT_START $VM_BOT_COUNT_END); do
  DISK_NAMES="$DISK_NAMES $PERSISTENT_DISK_NAME"-`printf "%03d" ${MACHINE_IP}`
done

gcloud compute --project $PROJECT_ID disks delete --quiet \
  --zone=$ZONE $DISK_NAMES
