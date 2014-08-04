#!/bin/bash
#
# Recovers slaves from VM crashes.
# Recovery commands that are not a part of the image yet should go in this
# script.
#
# The script should be run from the cluster-telemetry-master GCE instance's
# /b/skia-repo/buildbot/cluster_telemetry/telemetry_master_scripts
# directory.
#
# Copyright 2013 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)

source ../config.sh

# Update buildbot and trunk.
gclient sync

CRASHED_INSTANCES=""

# Modify the below script with packages necessary for the new images!
for SLAVE_NUM in $(seq 1 $NUM_SLAVES); do

  ssh -o ConnectTimeout=5 -o UserKnownHostsFile=/dev/null -o CheckHostIP=no \
    -o StrictHostKeyChecking=no \
    -A -q -p 22 build${SLAVE_NUM}-b5 -- "uptime" &> /dev/null
  if [ $? -ne 0 ]
  then
    echo "build$SLAVE_NUM-b5 is not responding!"
    CRASHED_INSTANCES="$CRASHED_INSTANCES build$SLAVE_NUM-b5"
  else
    OFFLINE_DEVICES=`ssh -o ConnectTimeout=5 -o UserKnownHostsFile=/dev/null -o CheckHostIP=no \
      -o StrictHostKeyChecking=no \
      -A -q -p 22 build${SLAVE_NUM}-b5 -- "adb devices | grep offline"`
    MISSING_DEVICES=`ssh -o ConnectTimeout=5 -o UserKnownHostsFile=/dev/null -o CheckHostIP=no \
      -o StrictHostKeyChecking=no \
      -A -q -p 22 build${SLAVE_NUM}-b5 -- "adb devices | grep device | grep -v devices"`
    if [ "$OFFLINE_DEVICES" != "" ]; then
      CRASHED_INSTANCES="$CRASHED_INSTANCES build$SLAVE_NUM-b5(N5 offline)"
    elif [ "$MISSING_DEVICES" == "" ]; then
      CRASHED_INSTANCES="$CRASHED_INSTANCES build$SLAVE_NUM-b5(N5 missing)"
    fi
  fi

done

if [[ $CRASHED_INSTANCES ]]; then
  echo "Emailing the administrator."
  BOUNDARY=`date +%s|md5sum`
  BOUNDARY=${BOUNDARY:0:32}
  sendmail $ADMIN_EMAIL <<EOF
subject:Some Cluster Telemetry instances crashed!
to:$ADMIN_EMAIL
from:skia.buildbot@gmail.com
Content-Type: multipart/mixed; boundary=\"$BOUNDARY\";

This is a MIME-encapsulated message

--$BOUNDARY
Content-Type: text/html

<html>
  <head/>
  <body>
The following instances crashed:<br/>
$CRASHED_INSTANCES<br/>
Please file a ticket to chrome-labs-tech-ticket@ <br/>
  </body>
</html>

--$BOUNDARY--

EOF

fi

