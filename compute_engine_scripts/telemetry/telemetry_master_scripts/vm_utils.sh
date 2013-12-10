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

function build_chromium {
  GYP_GENERATORS='ninja' ./build/gyp_chromium
  /home/default/depot_tools/ninja -C out/Release chrome
  if [ $? -ne 0 ]
  then
    echo "There was an error building chromium $CHROMIUM_COMMIT_HASH + skia $SKIA_COMMIT_HASH"
    exit 1
  fi
}

function copy_build_to_google_storage {
  dir_name=$1
  chromium_build_dir=$2
  # cd into the Release directory.
  cd $chromium_build_dir/src/out/Release
  # Move the not needed large directories.
  mv gen /tmp/
  mv obj /tmp/
  # Clean the directory in Google Storage.
  gsutil rm -R gs://chromium-skia-gm/telemetry/chromium-builds/$dir_name/*
  # Copy the newly built chrome binary into Google Storage.
  gsutil cp -r * gs://chromium-skia-gm/telemetry/chromium-builds/$dir_name/
  # Move the large directories back.
  mv /tmp/gen .
  mv /tmp/obj .
}

function reset_chromium_checkout {
  # TODO(rmistry): Investigate using gclient recurse to revert changes in all
  # checkouts.
  reset_cmd="git reset --hard HEAD; git clean -f -d"
  # Reset Skia.
  cd third_party/skia
  eval $reset_cmd
  # Reset Blink.
  cd ../WebKit
  eval $reset_cmd
  # Reset Chromium.
  cd ../..
  eval $reset_cmd
}
