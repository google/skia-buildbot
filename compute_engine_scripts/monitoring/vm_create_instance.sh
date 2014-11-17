#!/bin/bash
#
# Creates the compute instance for skiamonitor.com.
#
set -x

source vm_config.sh

gcutil --project=$PROJECT_ID addinstance $VM_NAME_BASE-monitoring \
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
  $VM_NAME_BASE-monitoring-data

gcutil --project=$PROJECT_ID attachdisk \
  --disk=$VM_NAME_BASE-monitoring-data \
  --zone=$ZONE $VM_NAME_BASE-monitoring
