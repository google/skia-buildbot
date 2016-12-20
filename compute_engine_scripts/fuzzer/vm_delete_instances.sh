#!/bin/bash
#
# Deletes the compute instance for skia-fuzzer.
#
set -x

source vm_config.sh

for name in ${ALL_FUZZER_INSTANCE_NAMES[@]}; do
  echo ${name}
  gcloud compute instances delete \
    --project=$PROJECT_ID \
    --delete-disks "all" \
    --zone=$ZONE \
    ${name}
done