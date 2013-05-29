#!/bin/bash
#
# Build chromium and skia ToT on the telemetry master instance and then push
# it to Google storage for the workers to consume.
#
# The script should be run from the skia-telemetry-master GCE instance's
# /home/default/skia-repo/buildbot/compute_engine_scripts/telemetry/telemetry_master_scripts
# directory.
#
# Copyright 2013 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)

# Update trunk.
cd ../../../../trunk/
gclient sync

# Run the script that checks out ToT chromium and skia and builds it.
tools/build-tot-chromium.sh /home/default/storage/chromium-trunk

# Copy to Google Storage only if chrome successfully built.
if [ -x /home/default/storage/chromium-trunk/src/out/Release/chrome ]
then
  # cd into the Release directory.
  cd /home/default/storage/chromium-trunk/src/out/Release
  # Delete the large subdirectories not needed to run the binary.
  rm -rf gen obj

  if [ -e /etc/boto.cfg ]; then
    # Move boto.cfg since it may interfere with the ~/.boto file.
    sudo mv /etc/boto.cfg /etc/boto.cfg.bak
  fi

  # Clean the directory in Google Storage.
  gsutil rm -R gs://chromium-skia-gm/telemetry/chrome-build/*
  # Copy the newly built chrome binary into Google Storage.
  gsutil cp -r * gs://chromium-skia-gm/telemetry/chrome-build/

fi

