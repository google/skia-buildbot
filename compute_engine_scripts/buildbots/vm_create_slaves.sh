#!/bin/bash
#
# Create all the Skia buildbot slave VM instances.
#
# Copyright 2013 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)

source vm_config.sh

COUNTER=0
# Get the IP addresses array for the requested zone.
IP_ADDRESSES_ARR=($(eval echo \$SLAVE_IP_ADDRESSES_${ZONE_TAG}))

for VM in $VM_SLAVE_NAMES; do
  IP_ADDRESS=${IP_ADDRESSES_ARR[$COUNTER]}
  COUNTER=$[$COUNTER +1]

  $GCOMPUTE_CMD addinstance ${VM_NAME_BASE}-${VM}-${ZONE_TAG} \
    --zone=$ZONE \
    --external_ip_address=$IP_ADDRESS \
    --service_account=default \
    --service_account_scopes="$SCOPES" \
    --network=default \
    --machine_type=$SLAVES_MACHINE_TYPE \
    --image=$SKIA_BUILDBOT_IMAGE_NAME \
    --nopersistent_boot_disk

  if [[ $? != "0" ]]; then
    echo
    echo "Creation of ${VM_NAME_BASE}-${VM}-${ZONE_TAG} failed!"
    exit 1
  fi
done

cat <<INP
If you did not see tables print out above then the vm names may be running
already. You will have to delete them to recreate them with different atttributes
or move them to a different zone.

Check ./vm_status.sh to wait until the status is RUNNING.

When the vm is ready, run vm_setup_slaves.sh to setup all created slaves.
INP
exit 0
