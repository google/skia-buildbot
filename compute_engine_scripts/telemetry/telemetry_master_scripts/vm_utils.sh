#!/bin/bash
#
# Utility functions for the telemetry master scripts.
#
# The script should be run from the skia-telemetry-master GCE instance's
# /home/default/skia-repo/buildbot/compute_engine_scripts/telemetry/telemetry_master_scripts
# directory.
#
# Copyright 2013 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)

function get_current_work_on_slave() {
  slave=$1
  current_work=`ssh -o UserKnownHostsFile=/dev/null -o CheckHostIP=no \
    -o StrictHostKeyChecking=no -i /home/default/.ssh/google_compute_engine \
    -A -q -p 22 default@108.170.222.$slave -- "ls /home/default/storage/current_work"`
  echo $current_work
}

function is_slave_currently_executing() {
  slave=$1
  work_of_interest=$2
  activities=$(get_current_work_on_slave $slave)
  arr=($activities)
  for work in ${arr[@]}; do
    if [[ $work_of_interest == $work ]]; then
      echo true
      exit 0
    fi
  done
  echo false
}

