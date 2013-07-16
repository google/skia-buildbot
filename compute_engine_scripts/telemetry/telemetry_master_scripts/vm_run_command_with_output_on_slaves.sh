#!/bin/bash
#
# Runs a specified command on all slaves.
#
# The script should be run from the skia-telemetry-master GCE instance's
# /home/default/skia-repo/buildbot/compute_engine_scripts/telemetry/telemetry_master_scripts
# directory.
#
# Copyright 2013 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)

source ../vm_config.sh

if [ $# -ne 1 ]; then
  echo
  echo "Usage: `basename $0` \"pkill -9 -f tools/perf/record_wpr\""
  echo
  echo "The first argument is the command that should be run on all the slaves."
  echo
  exit 1
fi

CMD=$1

# Update buildbot.
gclient sync

echo "About to run $CMD on all slaves..."
for SLAVE_NUM in $(seq 1 $NUM_SLAVES); do
  cmd_output=`ssh -o UserKnownHostsFile=/dev/null -o CheckHostIP=no \
    -o StrictHostKeyChecking=no -i /home/default/.ssh/google_compute_engine \
    -A -q -p 22 default@108.170.222.$SLAVE_NUM -- "$CMD"`
  if [ "$cmd_output" ]; then
    echo "===== skia-telemetry-worker$SLAVE_NUM output: ====="
    echo $cmd_output
    echo "============================================"
  fi
done

