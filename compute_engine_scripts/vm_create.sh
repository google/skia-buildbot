#!/bin/bash
#
# Create all the VMs instances
#
# You have to run this when bringing up new VMs or migrating VMs from one
# zone to another. Note that VM names are global across zones, so to migrate
# you may have to run vm_delete.sh first.
#
# Copyright 2012 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)

source vm_config.sh

SCOPES="https://www.googleapis.com/auth/devstorage.full_control"

# Find the free IP address possible to assign to new instances.
# NOTE: We only have to do this because our cluster has static IP addresses and
# bigcluster does not have a feature to auto assign those.

# Get a list of IP address that are available to the project
ALL_IPS=$(mktemp)
trap '/bin/rm $ALL_IPS ; exit 0' 0 1 2 3 15
$GCOMPUTE_CMD getproject 2>/dev/null \
  | sed -n -e 's/.* ips  *| *\(.*\) *|.*$/\1/p' \
  | tr , \\n \
  | sort >$ALL_IPS
# Remove trailing whitespace else it results in an incorrect comparison.
sed -i 's/[ \t]*$//' $ALL_IPS

# Get a list of IP addresss used so far
#   1. Grab the external_ip column.
#   2. Remove lines that are not IPs, strip spaces
#   3. Set difference against ALL_IPS to see what is free
FREE_IPS=$($GCOMPUTE_CMD listinstances 2>/dev/null \
  | awk -F\| '{ print $7; }' \
  | sed -n -e '/[0-9][0-9]*\.[0-9][0-9]*/s/ *//gp' \
  | sort | comm -23 $ALL_IPS -)

if [ -z "$FREE_IPS" ] ; then
  echo "No available IP addresses. Check quotas with \'gcutil getproject.\'" 1>&2
  exit 1
fi
echo Available IP addresses: $FREE_IPS

# Turn it into an array
FREE_IP_LIST=($FREE_IPS)
FREE_IP_INDEX=0

for VM in $VM_NAMES; do
  $GCOMPUTE_CMD addinstance ${VM_NAME_BASE}-${VM}-${ZONE_TAG} \
    --zone=$ZONE \
    --machine='standard-2-cpu' \
    --external_ip_address=${FREE_IP_LIST[$FREE_IP_INDEX]} \
    --service_account=default \
    --service_account_scopes="$SCOPES" \
    --disk=${VM}-disk-${ZONE_TAG} \
    --network=default
  FREE_IP_INDEX=$(expr $FREE_IP_INDEX + 1)
done

/bin/rm -f $TEMP_STARTUP_SCRIPT

cat <<INP
If you did not see a table which looked like
+---------------------+-------------------------------------------
| name                | operation-1327681189228-4b784dda81d58-b99dd05c |
| status              | DONE                                           |
| target              | ${VM}-${ZONE_TAG}
 ...
| operationType       | insert                                         |

Then the vm name may be running already. You will have to delete it to
recreate it with different atttributes or move it to a different zone.

Check ./vm_status.sh to wait until the status is RUNNING

When the vm is ready, run vm_setup_masters.sh and vm_setup_slaves.sh
INP
exit 0
