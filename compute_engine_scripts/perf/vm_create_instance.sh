#!/bin/bash
#
# Creates the compute instance for skia-perf.
#
set -x

source vm_config.sh

# Create a boot disk from the pushable base snapshot.
gcloud compute --project $PROJECT_ID disks create $INSTANCE_NAME \
  --zone $ZONE \
  --source-snapshot $PERF_SOURCE_SNAPSHOT \
  --type "pd-standard"

# Create a large data disk.
gcloud compute --project $PROJECT_ID disks create $INSTANCE_NAME"-data" \
  --size "1000" \
  --zone $ZONE \
  --type "pd-standard"

# Create the instance with the two disks attahed.
gcloud compute --project $PROJECT_ID instances create $INSTANCE_NAME \
  --zone $ZONE \
  --machine-type $PERF_MACHINE_TYPE \
  --network "default" \
  --maintenance-policy "MIGRATE" \
  --scopes $PERF_SCOPES \
  --tags "http-server" "https-server" \
  --metadata-from-file "startup-script=startup-script.sh" \
  --disk name=${INSTANCE_NAME}      device-name=${INSTANCE_NAME}      "mode=rw" "boot=yes" "auto-delete=yes" \
  --disk name=${INSTANCE_NAME}-data device-name=${INSTANCE_NAME}-data "mode=rw" "boot=no" \
  --address=$PERF_IP_ADDRESS

