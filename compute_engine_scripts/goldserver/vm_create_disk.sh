#!/bin/bash
#
#  Creates the data disk for skia gold for the specified instance.
#
set -x

VM_ID=$1
source vm_config.sh

# # Create a large data disk.
gcloud compute --project $PROJECT_ID disks create $GOLD_DATA_DISK_NAME \
  --size $GOLD_DATA_DISK_SIZE \
  --zone $ZONE \
  --type "pd-standard"
