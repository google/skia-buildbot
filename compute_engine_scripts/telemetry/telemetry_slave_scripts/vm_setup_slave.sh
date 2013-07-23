#!/bin/bash
#
# Sets up the slave to capture webpage archives or to run telemetry. Does the
# following:
# * Makes sure ~/.boto is used instead of /etc/boto.cfg.
# * Copies over the chrome binary in Google Storage.
# * Setups up the right symlinks to find google-chrome.
# * Copies over the page_sets for this slave from Google Storage.
# * Syncs the Skia buildbot checkout.
# * Creates an Xvfb display on :0.
#
# The script should be run from the skia-telemetry-slave GCE instance's
# /home/default/skia-repo/buildbot/compute_engine_scripts/telemetry/telemetry_slave_scripts
# directory.
#
# Copyright 2013 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)

if [ -e /etc/boto.cfg ]; then
  # Move boto.cfg since it may interfere with the ~/.boto file.
  sudo mv /etc/boto.cfg /etc/boto.cfg.bak
fi

# Download the chrome binary from Google Storage if the local TIMESTAMP is out
# of date.
mkdir -p /home/default/storage/chrome-build/
are_timestamps_equal /home/default/storage/chrome-build gs://chromium-skia-gm/telemetry/chrome-build
if [ $? -eq 1 ]; then
  rm -rf /tmp/default/storage/chrome-build*
  gsutil cp -R gs://chromium-skia-gm/telemetry/chrome-build/* \
    /home/default/storage/chrome-build/
fi

# Setup the right symlinks for telemetry.
sudo rm /usr/bin/google-chrome
sudo ln -s /home/default/storage/chrome-build/chrome /usr/bin/google-chrome
sudo chmod 777 /usr/bin/google-chrome

# Download the page_sets from Google Storage if the local TIMESTAMP is out of
# date.
mkdir -p /home/default/storage/page_sets/$PAGESETS_TYPE/
are_timestamps_equal /home/default/storage/page_sets/$PAGESETS_TYPE gs://chromium-skia-gm/telemetry/page_sets/slave$SLAVE_NUM/$PAGESETS_TYPE
if [ $? -eq 1 ]; then
  gsutil cp gs://chromium-skia-gm/telemetry/page_sets/slave$SLAVE_NUM/$PAGESETS_TYPE/* \
    /home/default/storage/page_sets/$PAGESETS_TYPE/
fi

# Create /etc/lsb-release which is needed by telemetry.
echo """
# $Id: //depot/ops/corp/puppet/goobuntu/common/modules/base/templates/lsb-release.erb#1 $
DISTRIB_CODENAME=precise
DISTRIB_DESCRIPTION="Ubuntu 12.04.2 LTS"
DISTRIB_ID=Ubuntu
DISTRIB_RELEASE=12.04
GOOGLE_CODENAME=precise
GOOGLE_ID=Goobuntu
GOOGLE_RELEASE="12.04 201305PD1-3"
GOOGLE_ROLE=desktop
GOOGLE_TRACK=stable
""" | sudo tee -a /etc/lsb-release

# Sync buildbot code to head.
cd /home/default/skia-repo/buildbot
/home/default/depot_tools/gclient sync

# Start an Xvfb display on :0.
sudo Xvfb :0 -screen 0 1280x1024x24 &
cd third_party/chromium_trunk/

