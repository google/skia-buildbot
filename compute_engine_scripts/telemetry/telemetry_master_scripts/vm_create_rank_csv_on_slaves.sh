#!/bin/bash
#
# Starts the telemetry_slave_scripts/vm_create_rank_csv.py script on all slaves.
#
# The script should be run from the skia-telemetry-master GCE instance's
# /home/default/skia-repo/buildbot/compute_engine_scripts/telemetry/telemetry_master_scripts
# directory.
#
# Copyright 2013 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)

source ../vm_config.sh

if [ -e /etc/boto.cfg ]; then
  # Move boto.cfg since it may interfere with the ~/.boto file.
  sudo mv /etc/boto.cfg /etc/boto.cfg.bak
fi

# Update buildbot.
# gclient sync

# Run vm_create_rank_csv.py on all the slaves.
for SLAVE_NUM in $(seq 1 $NUM_SLAVES); do
  CMD="python vm_create_rank_csv.py --slave_num=$SLAVE_NUM"
  ssh -f -X -o UserKnownHostsFile=/dev/null -o CheckHostIP=no \
    -o StrictHostKeyChecking=no -i /home/default/.ssh/google_compute_engine \
    -A -p 22 default@108.170.192.$SLAVE_NUM -- "source .bashrc; cd skia-repo/buildbot/compute_engine_scripts/telemetry/telemetry_slave_scripts; $CMD 2>&1"
done

# Check to see if all slaves are done
COMPLETED_COUNT=$( gsutil ls -l gs://chromium-skia-gm/telemetry/csv/*/*.csv | grep -v TOTAL | wc -l )
while [ $COMPLETED_COUNT -lt $NUM_SLAVES ]; do
  echo "$COMPLETED_COUNT are done with the create csv script, sleeping for 10 seconds."
  sleep 10
  COMPLETED_COUNT=$( gsutil ls -l gs://chromium-skia-gm/telemetry/csv/*/*.csv | grep -v TOTAL | wc -l )
done

# Copy everything locally and combine it into one file.
for SLAVE_NUM in $(seq 1 $NUM_SLAVES); do
  gsutil cp gs://chromium-skia-gm/telemetry/csv/slave$SLAVE_NUM/top-1m-$SLAVE_NUM.csv \
    /tmp/
  cat /tmp/top-1m-$SLAVE_NUM.csv >> /tmp/top-1m.csv
done

# Copy the consolidated file into Google Storage.
gsutil cp -a public-read /tmp/top-1m.csv \
  gs://chromium-skia-gm/telemetry/csv/consolidated-outputs/top-1m.csv

# Delete all tmp files.
rm -rf /tmp/top-1m*

