#!/bin/bash
#
# Utility functions for the telemetry slave scripts.
#
# The script should be run from the skia-telemetry-slave GCE instance's
# /home/default/skia-repo/buildbot/compute_engine_scripts/telemetry/telemetry_slave_scripts
# directory.
#
# Copyright 2013 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)

# Functions which can be called by the telemetry slave scripts to create or
# delete current_work files.
function create_worker_file {
  mkdir -p /home/default/storage/current_work/
  touch /home/default/storage/current_work/$1
}
function delete_worker_file {
  rm /home/default/storage/current_work/$1
}

# Function which can be called by the telemetry slave scripts to test for
# equality of local and Google Storage TIMESTAMP files. Returns 0 if timestamps
# are equal, else returns 1.
function are_timestamps_equal {
  local_dir=$1
  gs_dir=$2
  unique_id=`date +%s`

  # Check to see if the local TIMESTAMP exists.
  if [ ! -e $local_dir/TIMESTAMP ]; then
    return 1
  fi

  # Check to see if the remote TIMESTAMP exists.
  gsutil cp $gs_dir/TIMESTAMP /tmp/TIMESTAMP-$unique_id
  if [ $? -eq 1 ]; then
    return 1
  fi

  # Check to see if the two timestamp files are equal.
  if ! diff $local_dir/TIMESTAMP /tmp/TIMESTAMP-$unique_id > /dev/null; then
    return 1
  fi

  rm /tmp/TIMESTAMP-$unique_id
  return 0
}
