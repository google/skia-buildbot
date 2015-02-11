#!/bin/bash
#
# Creates the compute instance for skiagold.
#
set -x

source vm_config.sh

# Create a boot disk from the pushable base snapshot.
gcloud compute --project $PROJECT_ID disks create $INSTANCE_NAME \
  --zone $ZONE \
  --source-snapshot $GOLD_SOURCE_SNAPSHOT \
  --type "pd-standard"

# Create a large data disk.
gcloud compute --project $PROJECT_ID disks create $GOLD_DATA_DISK_NAME \
  --size $GOLD_DATA_DISK_SIZE \
  --zone $ZONE \
  --type "pd-standard"

gcloud compute --project $PROJECT_ID instances create $INSTANCE_NAME \
  --zone $ZONE \
  --machine-type $GOLD_MACHINE_TYPE \
  --network "default" \
  --maintenance-policy "MIGRATE" \
  --scopes $SCOPES \
  --tags "http-server" "https-server" \
  --disk name=${INSTANCE_NAME}      device-name=${INSTANCE_NAME}      "mode=rw" "boot=yes" "auto-delete=yes" \
  --disk name=${GOLD_DATA_DISK_NAME} device-name=${GOLD_DATA_DISK_NAME} "mode=rw" "boot=no" \
  --metadata-from-file "startup-script=startup-script.sh" \
  --address $GOLD_IP_ADDRESS
