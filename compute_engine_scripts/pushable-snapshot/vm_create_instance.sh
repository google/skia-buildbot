#!/bin/bash
#
# Creates the compute instance for taking pushable snapshot images.
#
set -x

source vm_config.sh

gcloud compute --project $PROJECT_ID instances create $INSTANCE_NAME \
  --zone $ZONE \
  --machine-type $MACHINE_TYPE \
  --network "default" \
  --maintenance-policy "MIGRATE" \
  --scopes $SCOPES \
  --tags "http-server" "https-server" \
  --metadata-from-file "startup-script=startup-script.sh" \
  --image $IMAGE_TYPE \
  --boot-disk-type "pd-standard" \
  --boot-disk-device-name $INSTANCE_NAME
