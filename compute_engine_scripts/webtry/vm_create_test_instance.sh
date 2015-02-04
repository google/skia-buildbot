#!/bin/bash
#
# Creates the compute instance for skfiddle.com
#
set -x

source ./vm_config.sh

gcutil --project=$PROJECT_ID addinstance $TEST_INSTANCE_NAME \
       --zone=$ZONE \
       --service_account=$PROJECT_USER \
       --service_account_scopes=$SCOPES \
       --network=default \
       --machine_type=$WEBTRY_MACHINE_TYPE \
       --image=$WEBTRY_IMAGE \
       --persistent_boot_disk
