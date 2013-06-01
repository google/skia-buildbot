#!/bin/bash
#
# Starts the telemetry_slave_scripts/vm_run_telemetry.sh script on all
# slaves.
#
# The script should be run from the skia-telemetry-master GCE instance's
# /home/default/skia-repo/buildbot/compute_engine_scripts/telemetry/telemetry_master_scripts
# directory.
#
# Copyright 2013 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)

if [ $# -ne 3 ]; then
  echo
  echo "Usage: `basename $0` skpicture_printer --skp-outdir=/home/default/storage/skps/ rmistry"
  echo
  echo "The first argument is the telemetry benchmark to run on this slave."
  echo "The second argument are the extra arguments that the benchmark needs."
  echo "The third argument is the user who triggered the run."
  echo
  exit 1
fi

TELEMETRY_BENCHMARK=$1
EXTRA_ARGS=$2
REQUESTOR=$3

source ../vm_config.sh
source vm_utils.sh 

# Update buildbot.
gclient sync

# Check if any slave is in the process of capturing archives.
for SLAVE_NUM in $(seq 1 $NUM_SLAVES); do
  result=$(is_slave_currently_executing $SLAVE_NUM $RECORD_WPR_ACTIVITY)
  if $result; then
    echo
    echo "skia-telemetry-worker$SLAVE_NUM is currently capturing archives!"
    echo "Please rerun this script after it is done."
    echo
    exit 1
  fi
done

for SLAVE_NUM in $(seq 1 $NUM_SLAVES); do
  CMD="bash vm_run_telemetry.sh $SLAVE_NUM $TELEMETRY_BENCHMARK $EXTRA_ARGS $REQUESTOR"
  ssh -f -X -o UserKnownHostsFile=/dev/null -o CheckHostIP=no \
    -o StrictHostKeyChecking=no -i /home/default/.ssh/google_compute_engine \
    -A -p 22 default@108.170.222.$SLAVE_NUM -- "source .bashrc; cd skia-repo/buildbot/compute_engine_scripts/telemetry/telemetry_slave_scripts; $CMD > /tmp/${TELEMETRY_BENCHMARK}_output.txt 2>&1"
done
