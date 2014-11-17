#!/bin/bash
#
# Deletes the compute instance for skiamonitor.com.
#
set -x

source vm_config.sh

gcutil --project=$PROJECT_ID deleteinstance \
  --zone=$ZONE $VM_NAME_BASE-monitoring

gcutil --project=$PROJECT_ID deletedisk \
  --zone=$ZONE $VM_NAME_BASE-monitoring-data
