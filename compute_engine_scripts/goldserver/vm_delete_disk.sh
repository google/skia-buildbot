#!/bin/bash
#
# Deletes the data disk for skia gold for the specified instance.
#
set -x

VM_ID=$1
source ./vm_config.sh

gcloud compute disks delete $GOLD_DATA_DISK_NAME \
  --project=$PROJECT_ID \
  --zone=$ZONE
