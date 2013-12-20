#!/bin/bash
#
# Create all the Skia buildbot master VM instance.
#
# Copyright 2013 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)

source vm_config.sh

COUNTER=0
# Get the IP addresses array for the requested zone.
IP_ADDRESSES_ARR=($(eval echo \$MASTER_IP_ADDRESSES_${ZONE_TAG}))

for VM in $VM_MASTER_NAMES; do
  IP_ADDRESS=${IP_ADDRESSES_ARR[$COUNTER]}
  COUNTER=$[$COUNTER +1]

  $GCOMPUTE_CMD addinstance ${VM_NAME_BASE}-${VM}-${ZONE_TAG} \
    --zone=$ZONE \
    --disk=${VM}-root-${ZONE_TAG},deviceName=master-root,boot \
    --disk=${VM}-disk-${ZONE_TAG},deviceName=master-disk \
    --external_ip_address=$IP_ADDRESS \
    --service_account=default \
    --service_account_scopes="$SCOPES" \
    --network=default \
    --machine_type=$MASTER_MACHINE_TYPE \
    --service_version=v1beta16

  if [[ $? != "0" ]]; then
    echo
    echo "Creation of ${VM_NAME_BASE}-${VM}-${ZONE_TAG} failed!"
    exit 1
  fi
done

cat <<INP
If you did not see a table print out above then the vm name may be running
already. You will have to delete it to recreate it with different attributes or
move it to a different zone.

Check ./vm_status.sh to wait until the status is RUNNING.

When the vm is ready, run vm_setup_masters.sh
INP
exit 0
