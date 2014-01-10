#!/bin/bash
#
# Copyright 2013 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)
#
# This scripts does the following:
# * Checks out Chromium ToT and Skia LKGR.
# * Applies the specified patch.
# * Builds Chromium.
# * Copies the build to a new directory.
# * Reverts the patch.
# * Builds Chromium.
# * Copies both builds to Google Storage.
#
# The script should be run from the skia-telemetry-master GCE instance's
# /home/default/skia-repo/buildbot/compute_engine_scripts/telemetry/telemetry_master_scripts
# directory.

if [ $# -ne 6 ]; then
  echo
  echo "Usage: `basename $0` /tmp/patch chromium rmistry-2013-11-20.07-34-05 /tmp/logfile 1"
  echo
  echo "The first argument is the location of the Chromium Patch."
  echo "The second argument is the location of the Blink Patch."
  echo "The third argument is the location of the Skia Patch."
  echo "The fourth argument is the unique runid (typically requester + timestamp)."
  echo "The fifth argument is the location of the log file."
  echo "The sixth argument is whether chromium should be built with aura. 1 means yes, 0 means no."
  echo
  exit 1
fi

CHROMIUM_PATCH_LOCATION=$1
BLINK_PATCH_LOCATION=$2
SKIA_PATCH_LOCATION=$3
RUN_ID=$4
LOG_FILE_LOCATION=$5
USE_AURA=$6

source vm_utils.sh

function copy_build_log_to_gs() {
  echo "== Copying build log to gs =="
  gsutil cp -a public-read $LOG_FILE_LOCATION \
    gs://chromium-skia-gm/telemetry/tryserver-outputs/build-logs/$RUN_ID-chromium.txt
  rm $LOG_FILE_LOCATION
}

cd ../../../slave/skia_slave_scripts/utils/
CHROMIUM_BUILD_DIR_BASE=/home/default/storage/chromium-builds/tryserver-base
mkdir -p $CHROMIUM_BUILD_DIR_BASE

# Find Chromium's ToT
git ls-remote https://chromium.googlesource.com/chromium/src.git --verify refs/heads/master &> /tmp/chromium-tot
CHROMIUM_COMMIT_HASH=`cut -f1 /tmp/chromium-tot`
# Find Skia's Git LKGR.
for i in {1..10}; do wget -O /tmp/skia-lkgr http://skia-tree-status.appspot.com/git-lkgr && break || sleep 2; done
SKIA_COMMIT_HASH=`cat /tmp/skia-lkgr`

# Ensure copying to Google Storage will work.
if [ -e /etc/boto.cfg ]; then
  # Move boto.cfg since it may interfere with the ~/.boto file.
  sudo mv /etc/boto.cfg /etc/boto.cfg.bak
fi

# Chromium sync command using Chromium ToT and Skia LKGR.
echo "== Syncing with chromium $CHROMIUM_COMMIT_HASH + skia $SKIA_COMMIT_HASH =="
SYNC_SKIA_IN_CHROME_CMD="PYTHONPATH=/home/default/skia-repo/buildbot/third_party/chromium_buildbot/site_config/:/home/default/skia-repo/buildbot/site_config/:/home/default/skia-repo/buildbot/third_party/chromium_buildbot/scripts/ python sync_skia_in_chrome.py --destination=$CHROMIUM_BUILD_DIR_BASE --chrome_revision=$CHROMIUM_COMMIT_HASH --skia_revision=$SKIA_COMMIT_HASH"

eval $SYNC_SKIA_IN_CHROME_CMD

if [ $? -ne 0 ]
then
  echo "== There was an error. Deleting base directory and trying again... =="
  rm -rf $CHROMIUM_BUILD_DIR_BASE
  mkdir -p $CHROMIUM_BUILD_DIR_BASE
  eval $SYNC_SKIA_IN_CHROME_CMD
fi
if [ $? -ne 0 ]
then
  echo "== There was an error checking out chromium $CHROMIUM_COMMIT_HASH + skia $SKIA_COMMIT_HASH =="
  copy_build_log_to_gs
  exit 1
fi

# Construct directory names from chromium and skia's truncated commit hashes.
DIR_NAME=try-${CHROMIUM_COMMIT_HASH:0:7}-${SKIA_COMMIT_HASH:0:7}-${RUN_ID}
DIR_NAME_WITH_PATCH=${DIR_NAME}-withpatch

cd $CHROMIUM_BUILD_DIR_BASE/src/

# Make sure we are starting from a clean slate.
reset_chromium_checkout

echo "== Applying the specified patches =="
# Stay in the current directory for the chromium patch.
apply_patch $CHROMIUM_PATCH_LOCATION
# Apply blink patch.
cd third_party/WebKit/
apply_patch $BLINK_PATCH_LOCATION
cd ../skia/
apply_patch $SKIA_PATCH_LOCATION

echo "== Building chromium with the patches =="
cd $CHROMIUM_BUILD_DIR_BASE/src/
build_chromium
echo "== Copy patch build to Google Storage =="
copy_build_to_google_storage $DIR_NAME_WITH_PATCH $CHROMIUM_BUILD_DIR_BASE

echo "== Building chromium without the patches =="
cd $CHROMIUM_BUILD_DIR_BASE/src/
reset_chromium_checkout
build_chromium
echo "== Copy build with no patch to Google Storage =="
copy_build_to_google_storage $DIR_NAME $CHROMIUM_BUILD_DIR_BASE

copy_build_log_to_gs
