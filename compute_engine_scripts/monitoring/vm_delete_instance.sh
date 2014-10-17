#!/bin/bash
#
# Delete the compute instance for skiamonitor.com.
#

source vm_config.sh

gcutil --project=$PROJECT_ID deleteinstance \
  --zone=$ZONE $VM_NAME_BASE-monitoring
gcutil --project=$PROJECT_ID deletedisk \
  --zone=$ZONE $VM_NAME_BASE-monitoring-data
