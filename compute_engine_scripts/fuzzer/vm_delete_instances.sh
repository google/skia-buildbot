#!/bin/bash
#
# Deletes the compute instance for skia-fuzzer.
#
set -x

source vm_config.sh

gcloud compute instances delete \
  --project=$PROJECT_ID \
  --delete-disks "all" \
  --zone=$ZONE \
  $FUZZER_FE_INSTANCE_NAME

for name in ${FUZZER_BE_INSTANCE_NAMES[@]}; do
  echo ${name}
  gcloud compute instances delete \
    --project=$PROJECT_ID \
    --delete-disks "all" \
    --zone=$ZONE \
    ${name}
done