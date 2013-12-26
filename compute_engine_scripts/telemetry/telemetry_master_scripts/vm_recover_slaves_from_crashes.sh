#!/bin/bash
#
# Recovers slaves from VM crashes.
# Recovery commands that are not a part of the image yet should go in this
# script.
#
# The script should be run from the skia-telemetry-master GCE instance's
# /home/default/skia-repo/buildbot/compute_engine_scripts/telemetry/telemetry_master_scripts
# directory.
#
# Copyright 2013 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)

source ../vm_config.sh

# Update buildbot and trunk.
gclient sync

CRASHED_INSTANCES=""

# Modify the below script with packages necessary for the new images!
for SLAVE_NUM in $(seq 1 $NUM_SLAVES); do

  ssh -o ConnectTimeout=5 -o UserKnownHostsFile=/dev/null -o CheckHostIP=no \
    -o StrictHostKeyChecking=no -i /home/default/.ssh/google_compute_engine \
    -A -q -p 22 default@108.170.222.$SLAVE_NUM -- "uptime" &> /dev/null
  if [ $? -ne 0 ]
  then
    echo "skia-telemetry-worker$SLAVE_NUM is not responding, deleting it."
    gcutil --project=google.com:chromecompute deleteinstance skia-telemetry-worker$SLAVE_NUM -f
    echo "Recreating skia-telemetry-worker$SLAVE_NUM"
    gcutil --project=google.com:chromecompute addinstance skia-telemetry-worker${SLAVE_NUM} \
      --zone=$ZONE \
      --service_account=default \
      --service_account_scopes="https://www.googleapis.com/auth/devstorage.full_control" \
      --network=skia \
      --image=skiatelemetry-3-0-v20131101 \
      --machine_type=n1-standard-8-d \
      --external_ip_address=108.170.222.${SLAVE_NUM} \
      --nopersistent_boot_disk \
      --service_version=v1beta16

    echo "Sleeping for 2 mins to give the instance time to come up and mount its scratch disk."
    sleep 120
  fi

  # After VM crashes the slaves only contain one 'lost+found' directory in the
  # mounted scratch disk. This is our indication that the VM crashed recently.
  NUM_DIRS=`ssh -o UserKnownHostsFile=/dev/null -o CheckHostIP=no \
    -o StrictHostKeyChecking=no -i /home/default/.ssh/google_compute_engine \
    -A -q -p 22 default@108.170.222.$SLAVE_NUM -- "ls -d ~/storage/*/ | wc -l"`
  if [ "$NUM_DIRS" == "1" ]; then
    echo "skia-telemetry-worker$SLAVE_NUM crashed! Recovering it..."
    CMD="""
sudo chmod 777 ~/.gsutil;
sudo ln -s /usr/bin/perf_3.2.0-55 /usr/sbin/perf;
cd ~/skia-repo;
rm -rf trunk;
sudo apt-get -y install python-imaging libosmesa-dev;
/home/default/depot_tools/gclient sync;
mkdir /home/default/storage/recovered;
"""
    ssh -f -X -o UserKnownHostsFile=/dev/null -o CheckHostIP=no \
      -o StrictHostKeyChecking=no -i /home/default/.ssh/google_compute_engine \
      -A -q -p 22 default@108.170.222.$SLAVE_NUM -- "$CMD"
    CRASHED_INSTANCES="$CRASHED_INSTANCES skia-telemetry-worker$SLAVE_NUM"
  else
    echo "skia-telemetry-worker$SLAVE_NUM has not crashed."
  fi
  echo "-----------------------------------------------------------"
done

if [[ $CRASHED_INSTANCES ]]; then
  echo "Emailing the administrator."
  BOUNDARY=`date +%s|md5sum`
  BOUNDARY=${BOUNDARY:0:32}
  sendmail $ADMIN_EMAIL <<EOF
subject:Some Cluster Telemetry instances crashed!
to:$ADMIN_EMAIL
from:skia.buildbot@gmail.com
Content-Type: multipart/mixed; boundary=\"$BOUNDARY\";

This is a MIME-encapsulated message

--$BOUNDARY
Content-Type: text/html

<html>
  <head/>
  <body>
The following instances crashed and have been recovered:<br/>
$CRASHED_INSTANCES
  </body>
</html>

--$BOUNDARY--

EOF

fi

