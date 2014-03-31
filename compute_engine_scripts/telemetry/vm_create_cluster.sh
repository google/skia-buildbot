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
  FREE_IP_LIST[$SLAVE_NUM]='108.170.192.'$SLAVE_NUM
done
FREE_IP_INDEX=0

# Create the telemetry master.
$GCOMPUTE_CMD addinstance ${VM_NAME_BASE}-${VM_MASTER_NAME} \
  --zone=$ZONE \
  --external_ip_address=${FREE_IP_LIST[$FREE_IP_INDEX]} \
  --service_account=default \
  --service_account_scopes="$SCOPES" \
  --network=skia \
  --image=skiatelemetry-6-0-ubuntu1310 \
  --machine_type=lmt-n1-standard-8-d \
  --auto_delete_boot_disk


# --external_ip_address=${FREE_IP_LIST[$FREE_IP_INDEX]} \
# --machine_type=n1-standard-1 \
# --image=debian-7-wheezy-v20140318 \
# --machine_type=lmt-n1-standard-8-d \
# --image=skiatelemetry-6-0-ubuntu1310 \

FREE_IP_INDEX=$(expr $FREE_IP_INDEX + 1)

# Create all telemetry slaves.
for SLAVE_NUM in $(seq 1 $NUM_SLAVES); do
  SLAVE_NAME=${VM_NAME_BASE}-${VM_SLAVE_NAME}${SLAVE_NUM}
  $GCOMPUTE_CMD addinstance ${SLAVE_NAME} \
    --zone=$ZONE \
    --service_account=default \
    --service_account_scopes="$SCOPES" \
    --network=skia \
    --image=skiatelemetry-6-0-ubuntu1310 \
    --machine_type=lmt-n1-standard-8-d \
    --external_ip_address=${FREE_IP_LIST[$FREE_IP_INDEX]} \
    --persistent_boot_disk \
    --auto_delete_boot_disk
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
  gcutil --project=google.com:chromecompute ssh --ssh_user=default cluster-telemetry-master
and run:
  * Make sure the scratch disk on the master is mounted correctly with 'df -v'.
  * Install the latest version of gcutil and then run
    gcutil --project=google.com:chromecompute ssh --ssh_user=default cluster-telemetry-worker1
to setup gcutil promptless authentication from the master to its workers.
  * Add a ~/.netrc by generating a new password from https://chromium.googlesource.com/
  * Set 'git config --global user.name' and 'git config --global user.email'
  * Run 'cd /home/default/skia-repo; gclient sync'
  * rm /home/default/google-cloud-sdk/bin/gsutil; ln -s /home/default/google-cloud-sdk/platform/gsutil/gsutil /home/default/google-cloud-sdk/bin/gsutil;
  * Install the following missing packages:
      sudo apt-get install python-django
  * sudo ln -s /home/default/google-cloud-sdk/bin/gsutil /usr/sbin/gsutil
  * sudo apt-get -y install haveged && sudo /etc/init.d/haveged start;
  * Run vm_recover_slaves_from_crashes.sh
  * Verify that all slaves are healthy by running:
      bash vm_run_command_with_output_on_slaves.sh "ls -l storage/"
  * Start the /home/default/skia-repo/buildbot/compute_engine_scripts/telemetry/telemetry_master_scripts/appengine_poller.py script.
INP

