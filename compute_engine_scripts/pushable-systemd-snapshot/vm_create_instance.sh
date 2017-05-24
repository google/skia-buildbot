#!/bin/bash
#
# Creates the compute instance for taking pushable snapshot images.
#
set -e -x

source vm_config.sh

gcloud compute --project $PROJECT_ID instances create $INSTANCE_NAME \
  --zone $ZONE \
  --machine-type $MACHINE_TYPE \
  --network "default" \
  --maintenance-policy "MIGRATE" \
  --image-family $IMAGE_FAMILY \
  --image-project $IMAGE_PROJECT \
  --boot-disk-type "pd-standard" \
  --boot-disk-device-name $INSTANCE_NAME \
  --address=$IP_ADDRESS

# Wait until the instance is up.
until nc -w 1 -z $IP_ADDRESS 22; do
    echo "Waiting for VM to come up."
    sleep 2
done

gcloud compute copy-files ./setup-script.sh default@${INSTANCE_NAME}:setup-script.sh \
  --zone $ZONE

# Debian is moving to whiptail by default which doesn't play well running the
# script remotely, so we SSH in directly and then run.
echo "Now ssh into ${INSTANCE_NAME} and run 'sudo ./setup-script.sh'"
echo ""
echo "gcloud compute ssh default@${INSTANCE_NAME} --zone $ZONE"
