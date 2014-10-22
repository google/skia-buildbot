#!/bin/bash
#
# Create the compute instance for skiaperf.com 
#
set -x

source ./vm_config.sh

gcutil --project=$PROJECT_ID addinstance $INSTANCE_NAME \
       --zone=$ZONE \
       --external_ip_address=$TESTING_IP_ADDRESS \
       --service_account=$PROJECT_USER \
       --service_account_scopes=$SCOPES \
       --network=default \
       --machine_type=$TESTING_MACHINE_TYPE \
       --image=$TESTING_IMAGE \
       --persistent_boot_disk
