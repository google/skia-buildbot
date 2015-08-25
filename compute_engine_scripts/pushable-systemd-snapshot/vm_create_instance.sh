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
  --tags "http-server,https-server" \
  --image $IMAGE_TYPE \
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

gcloud compute ssh default@${INSTANCE_NAME} --zone $ZONE \
  --command "sudo bash setup-script.sh"
