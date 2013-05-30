#!/bin/bash
#
# Runs all steps in vm_setup_slave.sh, executes record_wpr and copies the
# created archives to Google Storage.
#
# The script should be run from the skia-telemetry-slave GCE instance's
# /home/default/skia-repo/buildbot/compute_engine_scripts/telemetry/telemetry_slave_scripts
# directory.
#
# Copyright 2013 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)

if [ $# -ne 2 ]; then
  echo
  echo "Usage: `basename $0` 1 alexa1-10000.json"
  echo
  echo "The first argument is the slave_num of this telemetry slave."
  echo "The second argument is the page_set that should be processed."
  echo
  exit 1
fi

SLAVE_NUM=$1
PAGE_SET_FILENAME=$2

source ../vm_config.sh
source vm_utils.sh

create_worker_file $RECORD_WPR_ACTIVITY

source vm_setup_slave.sh

# Create the webpages_archive directory.
mkdir -p /home/default/storage/webpages_archive/
rm -rf /home/default/storage/webpages_archive/*

DISPLAY=:0 tools/perf/record_wpr --browser=system \
  /home/default/storage/page_sets/$PAGE_SET_FILENAME

# Copy the webpages_archive directory to Google Storage.
gsutil rm -R gs://chromium-skia-gm/telemetry/webpages_archive/slave$SLAVE_NUM/*
gsutil cp /home/default/storage/webpages_archive/* \
  gs://chromium-skia-gm/telemetry/webpages_archive/slave$SLAVE_NUM/

delete_worker_file $RECORD_WPR_ACTIVITY
