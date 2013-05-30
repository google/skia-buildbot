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
  touch /home/default/storage/current_work/$1
}
function delete_worker_file {
  rm /home/default/storage/current_work/$1
}

