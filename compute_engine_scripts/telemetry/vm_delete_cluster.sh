#!/bin/bash
#
# Delete all the Skia telemetry VMs instances
#
# Copyright 2013 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)

source vm_config.sh

# Construct string of all telemetry instances.
VM_INSTANCES=${VM_NAME_BASE}-${VM_MASTER_NAME}
for SLAVE_NUM in $(seq 1 $NUM_SLAVES); do
  VM_INSTANCES=$VM_INSTANCES" "${VM_NAME_BASE}-${VM_SLAVE_NAME}${SLAVE_NUM}
done

# Delete the telemetry master and all its slaves.
$GCOMPUTE_CMD deleteinstance ${VM_INSTANCES} --zone=$ZONE
