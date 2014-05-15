#!/bin/bash
#
# Sets up the slave to capture webpage archives or to run telemetry. Does the
# following:
# * Copies over the chrome binary from Google Storage.
# * Copies over the page_sets for this slave from Google Storage.
# * Syncs the Skia buildbot checkout.
# * Creates an Xvfb display on :0.
#
# The script should be run from the cluster-telemetry-slave GCE instance's
# /b/skia-repo/buildbot/cluster_telemetry/telemetry_slave_scripts
# directory.
#
# Copyright 2013 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)

# When the Cluster Telemetry bare-metal machines crash they are sometimes
# recreated with /tmp set to only root access.
sudo chmod 777 /tmp

# Download the chrome binary from Google Storage if the local TIMESTAMP is out
# of date.
mkdir -p /b/storage/chromium-builds/${CHROMIUM_BUILD_DIR}/
are_timestamps_equal /b/storage/chromium-builds/${CHROMIUM_BUILD_DIR} gs://chromium-skia-gm/telemetry/chromium-builds/${CHROMIUM_BUILD_DIR}
if [ $? -eq 1 ]; then
  rm -rf /b/storage/chromium-builds/${CHROMIUM_BUILD_DIR}*
  mkdir -p /b/storage/chromium-builds/${CHROMIUM_BUILD_DIR}/
  for i in {1..5}; do gsutil -m cp -R gs://chromium-skia-gm/telemetry/chromium-builds/${CHROMIUM_BUILD_DIR}/* /b/storage/chromium-builds/${CHROMIUM_BUILD_DIR}/ && break || sleep 5; done
  sudo chmod 777 /b/storage/chromium-builds/${CHROMIUM_BUILD_DIR}/chrome
fi

# Download the page_sets from Google Storage if the local TIMESTAMP is out of
# date.
mkdir -p /b/storage/page_sets/$PAGESETS_TYPE/
are_timestamps_equal /b/storage/page_sets/$PAGESETS_TYPE gs://chromium-skia-gm/telemetry/page_sets/slave$SLAVE_NUM/$PAGESETS_TYPE
if [ $? -eq 1 ]; then
  gsutil -m cp gs://chromium-skia-gm/telemetry/page_sets/slave$SLAVE_NUM/$PAGESETS_TYPE/* \
    /b/storage/page_sets/$PAGESETS_TYPE/
fi

# Set all access permissions for webpagereplay_logs/logs.txt
sudo chmod 777 /b/skia-repo/buildbot/third_party/src/webpagereplay_logs/logs.txt

# Sync buildbot code to head.
cd /b/skia-repo/buildbot
/b/depot_tools/gclient sync

# Start an Xvfb display on :0.
sudo Xvfb :0 -screen 0 1280x1024x24 &
cd third_party/chromium_trunk/

