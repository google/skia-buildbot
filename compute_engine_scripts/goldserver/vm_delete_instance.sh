#!/bin/bash
#
# Deletes the compute instance for skiagold.
# It does NOT delete the data disk which is deleted explicitly via
# vm_delete_disk.sh. 
#
set -x

source ./vm_config.sh

gcloud compute instances delete \
  --project=$PROJECT_ID \
  --zone=$ZONE \
  $INSTANCE_NAME
