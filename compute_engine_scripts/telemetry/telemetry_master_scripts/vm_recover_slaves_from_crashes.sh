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

# Modify the below script with packages necessary for the new images!
for SLAVE_NUM in $(seq 1 $NUM_SLAVES); do
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
"""
    ssh -f -X -o UserKnownHostsFile=/dev/null -o CheckHostIP=no \
      -o StrictHostKeyChecking=no -i /home/default/.ssh/google_compute_engine \
      -A -q -p 22 default@108.170.222.$SLAVE_NUM -- "$CMD"
  else
    echo "skia-telemetry-worker$SLAVE_NUM has not crashed."
  fi
done

