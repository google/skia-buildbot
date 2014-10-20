#!/bin/bash
#
# Build chromium LKGR and skia ToT on the telemetry master instance and then
# push it to Google storage for the workers to consume.
#
# The script should be run from the cluster-telemetry-master GCE instance's
# /b/skia-repo/buildbot/cluster_telemetry/telemetry_master_scripts
# directory.
#
# Copyright 2013 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)

if [ $# -ne 5 ]; then
  echo
  echo "Usage: `basename $0` a1234b c5678d rmistry@google.com 1001 /tmp/logfile"
  echo
  echo "The first argument is the Chromium commit hash."
  echo "The second argument is the Skia commit hash."
  echo "The third argument is the email address of the requester."
  echo "The fourth argument is the key of the appengine admin task."
  echo "The fifth argument is location of the log file that should be emailed."
  echo
  exit 1
fi

CHROMIUM_COMMIT_HASH=$1
SKIA_COMMIT_HASH=$2
REQUESTER_EMAIL=$3
APPENGINE_KEY=$4
LOG_FILE_LOCATION=$5

# Update buildbot.
gclient sync

CHROMIUM_BUILD_DIR_BASE=/b/storage/chromium-builds/base
mkdir -p $CHROMIUM_BUILD_DIR_BASE
SYNC_SKIA_IN_CHROME_CMD="PYTHONPATH=/b/skia-repo/buildbot/third_party/chromium_buildbot/site_config/:/b/skia-repo/buildbot/site_config/:/b/skia-repo/buildbot/third_party/chromium_buildbot/scripts/:/b/skia-repo/buildbot/common/ python sync_skia_in_chrome.py --destination=$CHROMIUM_BUILD_DIR_BASE --chrome_revision=$CHROMIUM_COMMIT_HASH --skia_revision=$SKIA_COMMIT_HASH"
eval $SYNC_SKIA_IN_CHROME_CMD


if [ $? -ne 0 ]
then
  echo "There was an error. Deleting base directory and trying again..."
  rm -rf $CHROMIUM_BUILD_DIR_BASE
  eval $SYNC_SKIA_IN_CHROME_CMD
fi

if [ $? -ne 0 ]
then
  echo "There was an error checking out chromium $CHROMIUM_COMMIT_HASH + skia $SKIA_COMMIT_HASH"
  SUBJECT_FAILURE_SUFFIX=" with checkout failures"
  CHROMIUM_REV_DATE="0"
else
  # Construct directory name from chromium and skia's truncated commit hashes.
  DIR_NAME=${CHROMIUM_COMMIT_HASH:0:7}-${SKIA_COMMIT_HASH:0:7}
  # This is the directory that will be uploaded to google storage.
  CHROMIUM_BUILD_DIR=/b/storage/chromium-builds/$DIR_NAME
  mkdir -p $CHROMIUM_BUILD_DIR
  cp -R ${CHROMIUM_BUILD_DIR_BASE}/* ${CHROMIUM_BUILD_DIR}/
  cd $CHROMIUM_BUILD_DIR/src/

  # Find when the requested Chromium revision was submitted to display in the
  # appengine web app.
  CHROMIUM_REV_DATE=`git log --pretty=format:"%at" -1`

  # Build chromium.
  GYP_GENERATORS='ninja' ./build/gyp_chromium
  /b/depot_tools/ninja -C out/Release chrome

  # Copy to Google Storage only if chromium successfully built.
  if [ $? -ne 0 ]
  then
    echo "There was an error building chromium $CHROMIUM_COMMIT_HASH + skia $SKIA_COMMIT_HASH"
    SUBJECT_FAILURE_SUFFIX=" with build failures"
  else
    # cd into the Release directory.
    cd $CHROMIUM_BUILD_DIR/src/out/Release
    # Delete the large subdirectories not needed to run the binary.
    rm -rf gen obj

    # Clean the directory in Google Storage.
    gsutil rm -R gs://chromium-skia-gm/telemetry/chromium-builds/${DIR_NAME}/*
    # Copy the newly built chrome binary into Google Storage.
    gsutil -m cp -r * gs://chromium-skia-gm/telemetry/chromium-builds/${DIR_NAME}/
    # Create a TIMESTAMP file and copy it to Google Storage.
    TIMESTAMP=`date +%s`
    echo $TIMESTAMP > /tmp/$TIMESTAMP
    gsutil cp /tmp/$TIMESTAMP gs://chromium-skia-gm/telemetry/chromium-builds/${DIR_NAME}/TIMESTAMP
    rm /tmp/$TIMESTAMP
  fi
fi

# Copy the log file to Google Storage.
gsutil cp -a public-read $LOG_FILE_LOCATION \
  gs://chromium-skia-gm/telemetry/admin-task-outputs/$REQUESTER_EMAIL-$APPENGINE_KEY-chromium.txt
rm $LOG_FILE_LOCATION

OUTPUT_LINK=https://storage.cloud.google.com/chromium-skia-gm/telemetry/admin-task-outputs/$REQUESTER_EMAIL-$APPENGINE_KEY-chromium.txt
BOUNDARY=`date +%s|md5sum`
BOUNDARY=${BOUNDARY:0:32}
sendmail $REQUESTER_EMAIL <<EOF
subject:Your Chrome Build has completed${SUBJECT_FAILURE_SUFFIX}!
to:$REQUESTER_EMAIL
from:skia.buildbot@gmail.com
Content-Type: multipart/mixed; boundary=\"$BOUNDARY\";

This is a MIME-encapsulated message

--$BOUNDARY
Content-Type: text/html

<html>
  <head/>
  <body>
  You had requested a build with the Chromium commit hash <a href='https://chromium.googlesource.com/chromium/src/+/${CHROMIUM_COMMIT_HASH}'>${CHROMIUM_COMMIT_HASH:0:7}</a> and the Skia commit hash <a href='https://skia.googlesource.com/skia/+/${SKIA_COMMIT_HASH}'>${SKIA_COMMIT_HASH:0:7}</a>.<br/>
  The build log is available <a href='$OUTPUT_LINK'>here</a>.<br/>
  You can schedule more runs <a href='https://skia-tree-status.appspot.com/skia-telemetry/chromium_builds'>here</a>.<br/><br/>
  Thanks!
  </body>
</html>

--$BOUNDARY--

EOF

# Mark this task as completed on AppEngine.
PASSWORD=`cat /b/skia-repo/buildbot/cluster_telemetry/telemetry_master_scripts/appengine_password.txt`
wget --post-data "key=$APPENGINE_KEY&password=$PASSWORD&chromium_rev_date=$CHROMIUM_REV_DATE&build_log_link=$OUTPUT_LINK" "https://skia-tree-status.appspot.com/skia-telemetry/update_chromium_build_tasks" -O /dev/null

