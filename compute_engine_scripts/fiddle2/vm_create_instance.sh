#!/bin/bash
#
# Creates the compute instance for skia-fiddle2.
#
set -x -e

source vm_config.sh

FIDDLE_MACHINE_TYPE=n1-standard-4
FIDDLE_SOURCE_SNAPSHOT=skia-systemd-pushable-base
FIDDLE_SCOPES='https://www.googleapis.com/auth/devstorage.full_control'

set +e
# Create a boot disk from the pushable base snapshot.
gcloud compute --project $PROJECT_ID disks create $INSTANCE_NAME \
  --zone $ZONE \
  --source-snapshot $FIDDLE_SOURCE_SNAPSHOT \
  --type "pd-standard"

# The cmd may fail if the disk already exists, which is fine.
# Create a large data disk.
gcloud compute --project $PROJECT_ID disks create $INSTANCE_NAME"-data" \
  --size "1000" \
  --zone $ZONE \
  --type "pd-standard"
set -e

# Create the instance with the two disks attached.
gcloud beta compute --project $PROJECT_ID instances create $INSTANCE_NAME \
  --zone $ZONE \
  --maintenance-policy "TERMINATE" \
  --machine-type $FIDDLE_MACHINE_TYPE \
  --network "default" \
  --scopes $FIDDLE_SCOPES \
  --tags "http-server,https-server" \
  --metadata-from-file "startup-script=startup-script.sh" \
  --metadata "owner_primary=jcgregorio" \
  --disk "name=${INSTANCE_NAME},device-name=${INSTANCE_NAME},mode=rw,boot=yes,auto-delete=yes" \
  --disk "name=${INSTANCE_NAME}-data,device-name=${INSTANCE_NAME}-data,mode=rw,boot=no"
#  --accelerator type=nvidia-tesla-k80,count=1 \

