#!/bin/bash
#
# Create all the Skia buildbot master VM instance.
#
# Copyright 2013 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)

source vm_config.sh

# Get the IP address for the requested zone.
IP_ADDRESS=$(eval echo \$MASTER_IP_ADDRESS_${ZONE_TAG})

$GCOMPUTE_CMD addinstance ${VM_NAME_BASE}-${VM_MASTER_NAME}-${ZONE_TAG} \
  --zone=$ZONE \
  --external_ip_address=$IP_ADDRESS \
  --service_account=default \
  --service_account_scopes="$SCOPES" \
  --network=default \
  --machine_type=$MASTER_MACHINE_TYPE \
  --image=$SKIA_BUILDBOT_IMAGE_NAME \
  --nopersistent_boot_disk

cat <<INP
If you did not see a table print out above then the vm name may be running
already. You will have to delete it to recreate it with different atttributes or
move it to a different zone.

Check ./vm_status.sh to wait until the status is RUNNING.

When the vm is ready, run vm_setup_master.sh
INP
exit 0
