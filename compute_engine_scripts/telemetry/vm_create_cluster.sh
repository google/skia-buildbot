#!/bin/bash
#
# Create all the Skia telemetry VM instances
#
# You have to run this when bringing up new VMs or migrating VMs from one
# zone to another. Note that VM names are global across zones, so to migrate
# you may have to run vm_delete_cluster.sh first.
#
# Copyright 2013 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)

source vm_config.sh

SCOPES="https://www.googleapis.com/auth/devstorage.full_control"

for SLAVE_NUM in $(seq 0 $NUM_SLAVES); do
  FREE_IP_LIST[$SLAVE_NUM]='108.170.222.'$SLAVE_NUM
done
FREE_IP_INDEX=0

# Create the telemetry master.
$GCOMPUTE_CMD addinstance ${VM_NAME_BASE}-${VM_MASTER_NAME} \
  --zone=$ZONE \
  --external_ip_address=${FREE_IP_LIST[$FREE_IP_INDEX]} \
  --service_account=default \
  --service_account_scopes="$SCOPES" \
  --network=skia \
  --image=skiatelemetry-1-0-v20130524 \
  --machine_type=n1-standard-8-d

FREE_IP_INDEX=$(expr $FREE_IP_INDEX + 1)

# Create all telemetry slaves.
for SLAVE_NUM in $(seq 1 $NUM_SLAVES); do
  SLAVE_NAME=${VM_NAME_BASE}-${VM_SLAVE_NAME}${SLAVE_NUM}
  $GCOMPUTE_CMD addinstance ${SLAVE_NAME} \
    --zone=$ZONE \
    --service_account=default \
    --service_account_scopes="$SCOPES" \
    --network=skia \
    --image=skiatelemetry-1-0-v20130524 \
    --machine_type=n1-standard-8-d \
    --external_ip_address=${FREE_IP_LIST[$FREE_IP_INDEX]}
  FREE_IP_INDEX=$(expr $FREE_IP_INDEX + 1)
done

cat <<INP
If you did not see a table which looked like
+---------------------+-------------------------------------------
| name                | operation-1327681189228-4b784dda81d58-b99dd05c |
| status              | DONE                                           |
| target              | ${VM}
 ...
| operationType       | insert                                         |

Then the vm name may be running already. You will have to delete it to
recreate it with different atttributes or move it to a different zone.

Check ./vm_status.sh to wait until the status is RUNNING


SSH into the master with:
  gcutil --project=google.com:chromecompute ssh --ssh_user=default skia-telemetry-master
and run:
  * gcutil --project=google.com:chromecompute ssh --ssh_user=default skia-telemetry-worker1
to setup gcutil promptless authentication from the master to its workers.
  * Follow the instructions in
https://developers.google.com/compute/docs/networking#mailserver using skia.buildbots@gmail.com
  * Run 'sudo mv /etc/boto.cfg /etc/boto.cfg.bak'
  * 'svn sync' /home/default/skia-repo/buildbot and enter the correct AppEngine password in /home/default/skia-repo/buildbot/compute_engine_scripts/telemetry/telemetry_master_scripts/appengine_password.txt
  * Run 'sudo apt-get update; sudo apt-get -y install libgif-dev libgl1-mesa-dev libglu-dev gdb' on the slaves using vm_run_command_on_slaves.sh. This step will not be required when the image is recaptured.
  * Update buildbot and trunk directory of all slaves by running vm_run_command_on_slaves.sh (Will have to 'rm -rf third_party/externals/*' in trunk).
  * Start the /home/default/skia-repo/buildbot/compute_engine_scripts/telemetry/telemetry_master_scripts/appengine_polly.py script.
  * Trigger 'Rebuild Chrome' from http://skia-tree-status.appspot.com/skia-telemetry/.

INP

