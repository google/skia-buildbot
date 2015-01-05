#!/bin/bash
#
# Creates the compute instance for the skia frontends.
#
set -x

source vm_config.sh


for NUM in $(seq 1 $NUM_INSTANCES); do

  gcloud compute --project $PROJECT_ID disks create $INSTANCE_NAME-$NUM \
    --zone $ZONE \
    --source-snapshot skia-pushable-base \
    --type "pd-standard"

  gcloud compute --project $PROJECT_ID instances create $INSTANCE_NAME-$NUM\
    --zone $ZONE \
    --machine-type $MACHINE_TYPE \
    --network "default" \
    --maintenance-policy "MIGRATE" \
    --scopes $SCOPES \
    --tags "http-server" "https-server" \
    --disk "name="$INSTANCE_NAME-$NUM "device-name="$INSTANCE_NAME-$NUM "mode=rw" "boot=yes" "auto-delete=yes" \
    --metadata-from-file "startup-script=startup-script.sh"

done
