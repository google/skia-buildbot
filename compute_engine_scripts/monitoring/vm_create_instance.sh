#!/bin/bash
#
# Create the compute instance for skiamonitor.com.
#

source ../buildbots/vm_config.sh

gcutil --project=$PROJECT_ID addinstance skia-monitoring-$ZONE_TAG \
                 --zone=$ZONE
                 --external_ip_address=$MONITORING_IP_ADDRESS \
                 --service_account=$PROJECT_USER \
                 --service_account_scopes=$SCOPES \
                 --network=default \
                 --machine_type=$MONITORING_MACHINE_TYPE \
                 --image=$MONITORING_IMAGE \
                 --persistent_boot_disk

gcutil --project=$PROJECT_ID attachdisk --disk=skia-monitoring-data-$ZONE_TAG skia-monitoring-$ZONE_TAG
