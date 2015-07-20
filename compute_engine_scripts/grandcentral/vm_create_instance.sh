#!/bin/bash
#
# Creates the compute instance for skia-grandcentral
#
set -x

source vm_config.sh

GRANDCENTRAL_MACHINE_TYPE=n1-highmem-16
GRANDCENTRAL_SOURCE_SNAPSHOT=skia-systemd-pushable-base
GRANDCENTRAL_SCOPES='https://www.googleapis.com/auth/devstorage.full_control'
GRANDCENTRAL_IP_ADDRESS=104.154.112.109

# Create a boot disk from the pushable base snapshot.
gcloud compute --project $PROJECT_ID disks create $INSTANCE_NAME \
  --zone $ZONE \
  --source-snapshot $GRANDCENTRAL_SOURCE_SNAPSHOT \
  --type "pd-standard"

# Create a large data disk.
gcloud compute --project $PROJECT_ID disks create $INSTANCE_NAME"-data" \
  --size "1000" \
  --zone $ZONE \
  --type "pd-standard"

# Create the instance with the two disks attached.
gcloud compute --project $PROJECT_ID instances create $INSTANCE_NAME \
  --zone $ZONE \
  --machine-type $GRANDCENTRAL_MACHINE_TYPE \
  --network "default" \
  --maintenance-policy "MIGRATE" \
  --scopes $GRANDCENTRAL_SCOPES \
  --tags "http-server" "https-server" \
  --metadata-from-file "startup-script=startup-script.sh" \
  --disk name=${INSTANCE_NAME}      device-name=${INSTANCE_NAME}      "mode=rw" "boot=yes" "auto-delete=yes" \
  --disk name=${INSTANCE_NAME}-data device-name=${INSTANCE_NAME}-data "mode=rw" "boot=no" \
  --address=$GRANDCENTRAL_IP_ADDRESS
