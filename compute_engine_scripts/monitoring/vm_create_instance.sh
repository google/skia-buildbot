#!/bin/bash
#
# Create the compute instance for skiamonitor.com.
#
set -x

source ../buildbots/vm_config.sh

gcutil --project=$PROJECT_ID addinstance skia-monitoring \
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
  skia-monitoring-data

gcutil --project=$PROJECT_ID attachdisk --disk=skia-monitoring-data --zone=$ZONE skia-monitoring
