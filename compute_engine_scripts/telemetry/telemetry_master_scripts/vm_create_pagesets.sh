#!/bin/bash
#
# Creates page_sets for each slave, stores them locally and also copies them to
# Google Storage.
#
# The script should be run from the skia-telemetry-master GCE instance's
# /home/default/skia-repo/buildbot/compute_engine_scripts/telemetry/telemetry_master_scripts
# directory.
#
# Copyright 2013 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)

source ../vm_config.sh

# Update buildbot.
gclient sync

# Move into the buildbot/tools directory.
cd ../../../tools
# Delete the old page_sets.
rm -rf page_sets/*.json
# CLean and create directories where page_sets will be stored.
rm -rf ~/storage/page_sets/*

# Loop through all slaves and create only those many numbers.
# Copy them over to Google Storage
NUM_PAGESETS=$(($NUM_WEBPAGES/$NUM_SLAVES))
START=1
for SLAVE_NUM in $(seq 1 $NUM_SLAVES); do
  END=$(expr $START + $NUM_PAGESETS - 1)
  python create_page_set.py -s $START -e $END
  START=$(expr $END + 1)
  # Copy page_sets to the local directory.
  mkdir -p ~/storage/page_sets/slave$SLAVE_NUM
  mv page_sets/*.json ~/storage/page_sets/slave$SLAVE_NUM
done


if [ -e /etc/boto.cfg ]; then
  # Move boto.cfg since it may interfere with the ~/.boto file.
  sudo mv /etc/boto.cfg /etc/boto.cfg.bak
fi

# Clean the directory in Google Storage.
gsutil rm -R gs://chromium-skia-gm/telemetry/page_sets/*
# Copy the page_sets into Google Storage.
gsutil cp -r ~/storage/page_sets/* gs://chromium-skia-gm/telemetry/page_sets/

