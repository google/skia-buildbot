#!/bin/bash
#
# Deletes the compute instance for skiamonitor.com.
#
set -x

source vm_config.sh

gcutil --project=$PROJECT_ID deleteinstance \
  --zone=$ZONE $INSTANCE_NAME

gcutil --project=$PROJECT_ID deletedisk \
  --zone=$ZONE $DISK_NAME
