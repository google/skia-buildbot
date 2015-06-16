#!/bin/bash
#
# Creates the compute instance for skia-ctfe.
#
set -x

source vm_config.sh

# Create a boot disk from the pushable base snapshot.
gcloud compute --project $PROJECT_ID disks create $INSTANCE_NAME \
  --zone $ZONE \
  --source-snapshot $CTFE_SOURCE_SNAPSHOT \
  --type "pd-standard"

gcloud compute --project $PROJECT_ID instances create $INSTANCE_NAME \
  --zone $ZONE \
  --machine-type $CTFE_MACHINE_TYPE \
  --network "default" \
  --maintenance-policy "MIGRATE" \
  --scopes $CTFE_SCOPES \
  --tags "http-server" "https-server" \
  --disk "name=skia-ctfe" "device-name=skia-ctfe" "mode=rw" "boot=yes" "auto-delete=yes" \
  --address $CTFE_IP_ADDRESS

