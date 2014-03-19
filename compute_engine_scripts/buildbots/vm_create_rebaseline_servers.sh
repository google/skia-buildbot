#!/bin/bash
#
# Create the Skia rebaseline_server VM instance(s).
#
# Copyright 2014 Google Inc. All Rights Reserved.
# Author: epoger@google.com (Elliot Poger)

source vm_config.sh

COUNTER=0
# Get the IP addresses array for the requested zone.
IP_ADDRESSES_ARR=($(eval echo \$REBASELINESERVER_IP_ADDRESSES_${ZONE_TAG}))

for VM in $VM_REBASELINESERVER_NAMES; do
  IP_ADDRESS=${IP_ADDRESSES_ARR[$COUNTER]}
  COUNTER=$[$COUNTER +1]

  COMMAND="$GCOMPUTE_CMD addinstance ${VM_NAME_BASE}-${VM}-${ZONE_TAG} \
    --zone=$ZONE \
    --external_ip_address=$IP_ADDRESS \
    --service_account=default \
    --service_account_scopes="$SCOPES" \
    --network=default \
    --image=$SKIA_BUILDBOT_IMAGE_NAME_V1 \
    --machine_type=$REBASELINESERVER_MACHINE_TYPE \
    --persistent_boot_disk"
  echo $COMMAND
  $COMMAND

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

When the vm is ready, run vm_setup_rebaseline_servers.sh
INP
exit 0
