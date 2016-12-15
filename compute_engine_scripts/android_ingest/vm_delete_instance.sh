#!/bin/bash
#
# Deletes the compute instance for skia-android-ingest.
#
set -x

source vm_config.sh

gcloud compute instances delete \
  --project=$PROJECT_ID \
  --zone=$ZONE \
  $INSTANCE_NAME
