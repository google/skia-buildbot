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


if [ $# -ne 3 ]; then
  echo
  echo "Usage: `basename $0` 1 All a1234b-c5678d"
  echo
  echo "The first argument is the slave_num of this telemetry slave."
  echo "The second argument is the type of pagesets to create from the 1M list"\
       "Eg: All, Filtered, 100k, 10k, Deeplinks."
  echo "The third argument is the name of the directory where the chromium" \
       "build which will be used for this run is stored."
  echo
  exit 1
fi

SLAVE_NUM=$1
PAGESETS_TYPE=$2
CHROMIUM_BUILD_DIR=$3

source ../vm_config.sh
source vm_utils.sh

create_worker_file $RECORD_WPR_ACTIVITY

source vm_setup_slave.sh

# Create the webpages_archive directory.
mkdir -p /home/default/storage/webpages_archive/$PAGESETS_TYPE/
rm -rf /home/default/storage/webpages_archive/$PAGESETS_TYPE/*

for page_set in /home/default/storage/page_sets/$PAGESETS_TYPE/*; do
  if [[ -f $page_set ]]; then
    echo "========== Processing $page_set =========="
    pageset_basename=`basename $page_set`
    if [ "$PAGESETS_TYPE" == "Filtered" ]; then
      # Since the archive already exists in 'All' do not run record_wpr.
      pageset_filename="${pageset_basename%.*}"
      cp  /home/default/storage/webpages_archive/All/${pageset_filename}* /home/default/storage/webpages_archive/$PAGESETS_TYPE/
      echo "========== $page_set copied over from All =========="
    else
      check_and_run_xvfb
      DISPLAY=:0 timeout 300 src/tools/perf/record_wpr --extra-browser-args=--disable-setuid-sandbox --browser-executable=/home/default/storage/chromium-builds/${CHROMIUM_BUILD_DIR}/chrome --browser=exact $page_set
      if [ $? -eq 124 ]; then
        echo "========== $page_set timed out! =========="
      else
        echo "========== Done with $page_set =========="
      fi
    fi
  fi
done

# Copy the webpages_archive directory to Google Storage.
gsutil rm -R gs://chromium-skia-gm/telemetry/webpages_archive/slave$SLAVE_NUM/$PAGESETS_TYPE/*
gsutil cp /home/default/storage/webpages_archive/$PAGESETS_TYPE/* \
  gs://chromium-skia-gm/telemetry/webpages_archive/slave$SLAVE_NUM/$PAGESETS_TYPE/

# Create a TIMESTAMP file and copy it to Google Storage.
TIMESTAMP=`date +%s`
echo $TIMESTAMP > /tmp/$TIMESTAMP
cp /tmp/$TIMESTAMP /home/default/storage/webpages_archive/$PAGESETS_TYPE/TIMESTAMP
gsutil cp /tmp/$TIMESTAMP gs://chromium-skia-gm/telemetry/webpages_archive/slave$SLAVE_NUM/$PAGESETS_TYPE/TIMESTAMP
rm /tmp/$TIMESTAMP

delete_worker_file $RECORD_WPR_ACTIVITY
