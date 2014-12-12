#!/bin/bash
#
# Creates the compute instance for skia-push.
#
set -x

source vm_config.sh

# Create a boot disk from the pushable base snapshot.
gcloud compute --project $PROJECT_ID disks create $INSTANCE_NAME \
  --zone $ZONE \
  --source-snapshot $PUSH_SOURCE_SNAPSHOT \
  --type "pd-standard"

gcloud compute --project $PROJECT_ID instances create $INSTANCE_NAME \
  --zone $ZONE \
  --machine-type $PUSH_MACHINE_TYPE \
  --network "default" \
  --maintenance-policy "MIGRATE" \
  --scopes $PUSH_SCOPES \
  --tags "http-server" "https-server" \
  --disk "name=skia-push" "device-name=skia-push" "mode=rw" "boot=yes" "auto-delete=yes" \
  --address $PUSH_IP_ADDRESS

