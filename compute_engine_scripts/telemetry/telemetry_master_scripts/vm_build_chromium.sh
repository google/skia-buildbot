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

if [ $# -ne 3 ]; then
  echo
  echo "Usage: `basename $0` rmistry@google.com 1001 /tmp/logfile"
  echo
  echo "The first argument is the email address of the requester."
  echo "The second argument is the key of the appengine admin task."
  echo "The third argument is location of the log file that should be emailed."
  echo
  exit 1
fi

REQUESTER_EMAIL=$1
APPENGINE_KEY=$2
LOG_FILE_LOCATION=$3

# Update buildbot.
gclient sync

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
  # Create a TIMESTAMP file and copy it to Google Storage.
  TIMESTAMP=`date +%s`
  echo $TIMESTAMP > /tmp/$TIMESTAMP
  gsutil cp /tmp/$TIMESTAMP gs://chromium-skia-gm/telemetry/chrome-build/TIMESTAMP
  rm /tmp/$TIMESTAMP
fi

# Copy the log file to Google Storage.
gsutil cp -a public-read $LOG_FILE_LOCATION \
  gs://chromium-skia-gm/telemetry/admin-task-outputs/$REQUESTER_EMAIL-chromium.txt
rm $LOG_FILE_LOCATION

OUTPUT_LINK=https://storage.cloud.google.com/chromium-skia-gm/telemetry/admin-task-outputs/$REQUESTER_EMAIL-chromium.txt
BOUNDARY=`date +%s|md5sum`
BOUNDARY=${BOUNDARY:0:32}
sendmail $REQUESTER_EMAIL <<EOF
subject:Your Chrome Build has completed!
to:$REQUESTER_EMAIL
from:skia.buildbot@gmail.com
Content-Type: multipart/mixed; boundary=\"$BOUNDARY\";

This is a MIME-encapsulated message

--$BOUNDARY
Content-Type: text/html

<html>
  <head/>
  <body>
  The output of the script is available <a href='$OUTPUT_LINK'>here</a>.<br/>
  You can schedule more runs <a href='https://skia-tree-status.appspot.com/skia-telemetry/admin_tasks'>here</a>.<br/><br/>
  Thanks!
  </body>
</html>

--$BOUNDARY--

EOF

# Mark this task as completed on AppEngine.
PASSWORD=`cat /home/default/skia-repo/buildbot/compute_engine_scripts/telemetry/telemetry_master_scripts/appengine_password.txt`
wget --post-data "key=$APPENGINE_KEY&password=$PASSWORD" "https://skia-tree-status.appspot.com/skia-telemetry/update_admin_task" -O /dev/null

