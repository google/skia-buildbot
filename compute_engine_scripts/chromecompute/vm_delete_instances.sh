#!/bin/bash
#
# Delete the specified VM instances.
#
# Copyright 2014 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)

source vm_config.sh

for MACHINE_IP in $(seq $VM_BOT_COUNT_START $VM_BOT_COUNT_END); do
  VM_INSTANCES=$VM_INSTANCES" "${VM_BOT_NAME}-`printf "%03d" ${MACHINE_IP}`
done

$GCOMPUTE_CMD deleteinstance ${VM_INSTANCES} --zone=$ZONE --delete_boot_pd -f

