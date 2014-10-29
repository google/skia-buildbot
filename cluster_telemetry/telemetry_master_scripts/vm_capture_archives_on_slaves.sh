#!/bin/bash
#
# Starts the telemetry_slave_scripts/vm_capture_archives.sh script on all
# slaves.
#
# The script should be run from the cluster-telemetry-master GCE instance's
# /b/skia-repo/buildbot/cluster_telemetry/telemetry_master_scripts
# directory.
#
# Copyright 2013 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)

if [ $# -ne 4 ]; then
  echo
  echo "Usage: `basename $0` rmistry@google.com 1001 All a1234b-c5678d"
  echo
  echo "The first argument is the email address of the requester."
  echo "The second argument is the key of the appengine admin task."
  echo "The third argument is the type of pagesets to create from the 1M list" \
       "Eg: All, Filtered, 100k, 10k, Deeplinks."
  echo "The fourth argument is the name of the directory where the chromium" \
       "build which will be used for this run is stored."
  echo
  exit 1
fi

REQUESTER_EMAIL=$1
APPENGINE_KEY=$2
PAGESETS_TYPE=$3
CHROMIUM_BUILD_DIR=$4

source ../config.sh
source vm_utils.sh

# Update buildbot.
gclient sync

# Check if any slave is in the process of capturing archives.
for SLAVE_NUM in $(seq 1 $NUM_SLAVES); do
  result=$(is_slave_currently_executing $SLAVE_NUM $RECORD_WPR_ACTIVITY)
  if $result; then
    echo
    echo "cluster-telemetry-worker$SLAVE_NUM is currently capturing archives!"
    echo "Please rerun this script after it is done."
    echo
    exit 1
  fi
done

for SLAVE_NUM in $(seq 1 $NUM_SLAVES); do
  CMD="bash vm_capture_archives.sh $SLAVE_NUM $PAGESETS_TYPE $CHROMIUM_BUILD_DIR"
  ssh -f -X -o UserKnownHostsFile=/dev/null -o CheckHostIP=no \
    -o StrictHostKeyChecking=no \
    -A -p 22 build${SLAVE_NUM}-b5 -- "source .bashrc; cd /b/skia-repo/buildbot/cluster_telemetry/telemetry_slave_scripts; /b/depot_tools/gclient sync; $CMD > /tmp/capture_archives_output.txt 2>&1"
done

# Sleep for 2 minutes to give the slaves some time to start processing.
sleep 120

# Check to see if the slaves are done capturing archives.
SLAVES_STILL_PROCESSING=true
while $SLAVES_STILL_PROCESSING ; do
  SLAVES_STILL_PROCESSING=false
  for SLAVE_NUM in $(seq 1 $NUM_SLAVES); do
    RET=$( is_slave_currently_executing $SLAVE_NUM $RECORD_WPR_ACTIVITY )
    if $RET; then
      echo "cluster-telemetry-worker$SLAVE_NUM is still running $RECORD_WPR_ACTIVITY"
      echo "Sleeping for a minute and then retrying"
      SLAVES_STILL_PROCESSING=true
      sleep 60
      break
    else
      echo "cluster-telemetry-worker$SLAVE_NUM is done processing."
    fi
  done
done

BOUNDARY=`date +%s|md5sum`
BOUNDARY=${BOUNDARY:0:32}
sendmail $REQUESTER_EMAIL <<EOF
subject:Your Recreate Webpage Archives task has completed!
to:$REQUESTER_EMAIL
Content-Type: multipart/mixed; boundary=\"$BOUNDARY\";

This is a MIME-encapsulated message

--$BOUNDARY
Content-Type: text/html

<html>
  <head/>
  <body>
  You can schedule more runs <a href='https://skia-tree-status.appspot.com/skia-telemetry/admin_tasks'>here</a>.<br/><br/>
  Thanks!
  </body>
</html>

--$BOUNDARY--

EOF

# Mark this task as completed on AppEngine.
PASSWORD=`cat /b/skia-repo/buildbot/cluster_telemetry/telemetry_master_scripts/appengine_password.txt`
wget --post-data "key=$APPENGINE_KEY&password=$PASSWORD" "https://skia-tree-status.appspot.com/skia-telemetry/update_admin_task" -O /dev/null
