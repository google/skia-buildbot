#!/bin/bash
#
# Creates the compute instance for skiamonitor.com.
#
set -x

source vm_config.sh

gcutil --project=$PROJECT_ID addinstance $INSTANCE_NAME \
  --zone=$ZONE \
  --external_ip_address=$MONITORING_IP_ADDRESS \
  --service_account=$PROJECT_USER \
  --service_account_scopes=$SCOPES \
  --network=default \
  --machine_type=$MONITORING_MACHINE_TYPE \
  --image=$MONITORING_IMAGE \
  --persistent_boot_disk

gcutil --project=$PROJECT_ID adddisk \
  --description="Influx Data" \
  --disk_type=pd-standard \
  --size_gb=500 \
  --zone=$ZONE \
  $DISK_NAME

gcutil --project=$PROJECT_ID attachdisk \
  --disk=$DISK_NAME \
  --zone=$ZONE $INSTANCE_NAME
