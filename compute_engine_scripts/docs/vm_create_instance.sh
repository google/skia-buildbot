#!/bin/bash
#
# Creates the compute instance for skiadocs.
#
set -x

source vm_config.sh

# Create a boot disk from the pushable base snapshot.
gcloud compute --project $PROJECT_ID disks create $INSTANCE_NAME \
  --zone $ZONE \
  --source-snapshot skia-pushable-base \
  --type "pd-standard"

gcloud compute --project $PROJECT_ID instances create $INSTANCE_NAME \
  --zone $ZONE \
  --machine-type $MACHINE_TYPE \
  --network "default" \
  --maintenance-policy "MIGRATE" \
  --scopes $SCOPES \
  --tags "http-server" "https-server" \
  --disk "name="$INSTANCE_NAME "device-name="$INSTANCE_NAME "mode=rw" "boot=yes" "auto-delete=yes" \
  --metadata-from-file "startup-script=startup-script.sh" \
  --address $DOCS_IP_ADDRESS
