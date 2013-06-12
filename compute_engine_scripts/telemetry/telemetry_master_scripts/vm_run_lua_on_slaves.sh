#!/bin/bash
#
# Starts the telemetry_slave_scripts/vm_run_run_on_skps.sh script on all slaves.
#
# The script should be run from the skia-telemetry-master GCE instance's
# /home/default/skia-repo/buildbot/compute_engine_scripts/telemetry/telemetry_master_scripts
# directory.
#
# Copyright 2013 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)

if [ $# -ne 4 ]; then
  echo
  echo "Usage: `basename $0` /tmp/test.lua" \
       "rmistry-2013-05-24.07-34-05 rmistry@google.com 1001"
  echo
  echo "The first argument is the Google Storage location of the Lua script."
  echo "The second argument is a unique run id (typically requester + timestamp)."
  echo "The third argument is the email address of the requester."
  echo "The fourth argument is the key of the appengine lua task."
  echo
  exit 1
fi

LUA_SCRIPT_LOCAL_LOCATION=$1
RUN_ID=$2
REQUESTER_EMAIL=$3
APPENGINE_KEY=$4

source ../vm_config.sh

# Start the timer.
TIMER="$(date +%s)"

if [ -e /etc/boto.cfg ]; then
  # Move boto.cfg since it may interfere with the ~/.boto file.
  sudo mv /etc/boto.cfg /etc/boto.cfg.bak
fi

# Copy the local lua script to Google Storage.
LUA_SCRIPT_GS_LOCATION=gs://chromium-skia-gm/telemetry/lua-scripts/$RUN_ID.lua.txt
gsutil cp -a public-read $LUA_SCRIPT_LOCAL_LOCATION $LUA_SCRIPT_GS_LOCATION 

# Update buildbot.
gclient sync

# Run vm_run_lua_on_skps.sh on all the slaves.
for SLAVE_NUM in $(seq 1 $NUM_SLAVES); do
  CMD="bash vm_run_lua_on_skps.sh $SLAVE_NUM $LUA_SCRIPT_GS_LOCATION $RUN_ID"
  ssh -f -X -o UserKnownHostsFile=/dev/null -o CheckHostIP=no \
    -o StrictHostKeyChecking=no -i /home/default/.ssh/google_compute_engine \
    -A -p 22 default@108.170.222.$SLAVE_NUM -- "source .bashrc; cd skia-repo/buildbot/compute_engine_scripts/telemetry/telemetry_slave_scripts; $CMD > /tmp/lua_output.$RUN_ID.txt 2>&1"
done

# Check to see if all slaves are done
COMPLETED_COUNT=$( gsutil ls -l gs://chromium-skia-gm/telemetry/lua-outputs/*/${RUN_ID}.lua-output | grep -v TOTAL | wc -l )
while [ $COMPLETED_COUNT -lt $NUM_SLAVES ]; do
  echo "$COMPLETED_COUNT are done with the lua script, sleeping for 10 seconds."
  sleep 10
  COMPLETED_COUNT=$( gsutil ls -l gs://chromium-skia-gm/telemetry/lua-outputs/*/$RUN_ID.lua-output | grep -v TOTAL | wc -l )
done

# Copy everything locally and combine it into one file.
for SLAVE_NUM in $(seq 1 $NUM_SLAVES); do
  gsutil cp gs://chromium-skia-gm/telemetry/lua-outputs/slave$SLAVE_NUM/$RUN_ID.lua-output \
    /tmp/$RUN_ID-$SLAVE_NUM.lua-output
  cat /tmp/$RUN_ID-$SLAVE_NUM.lua-output >> /tmp/$RUN_ID.lua-output
done

# Copy the consolidated file into Google Storage.
gsutil cp -a public-read /tmp/$RUN_ID.lua-output \
  gs://chromium-skia-gm/telemetry/lua-outputs/consolidated-outputs/$RUN_ID.lua-output.txt

# Delete all tmp files.
rm -rf /tmp/$RUN_ID*

# End the timer.
TIMER="$(($(date +%s)-TIMER))"

# Email results to the requester and admins.
ADMINS=rmistry@google.com
OUTPUT_LINK=https://storage.cloud.google.com/chromium-skia-gm/telemetry/lua-outputs/consolidated-outputs/$RUN_ID.lua-output.txt
SCRIPT_LINK=https://storage.cloud.google.com/chromium-skia-gm/telemetry/lua-scripts/$RUN_ID.lua.txt
BOUNDARY=`date +%s|md5sum`
BOUNDARY=${BOUNDARY:0:32}
sendmail $REQUESTER_EMAIL,$ADMINS <<EOF
subject:Results of your Lua script run
to:$REQUESTER_EMAIL,$ADMINS
from:skia.buildbot@gmail.com
Content-Type: multipart/mixed; boundary=\"$BOUNDARY\";

This is a MIME-encapsulated message

--$BOUNDARY
Content-Type: text/html

<html>
  <head/>
  <body>
  Time taken for the <a href='$SCRIPT_LINK'>script</a> run: $TIMER seconds.<br/>
  The output of your script is available <a href='$OUTPUT_LINK'>here</a>.<br/>
  You can schedule more runs <a href='https://skia-tree-status-staging.appspot.com/skia-telemetry/lua_script'>here</a>.<br/><br/>
  Thanks!
  </body>
</html>

--$BOUNDARY--

EOF

# Mark this task as completed on AppEngine.
wget -o /dev/null "http://skia-tree-status-staging.appspot.com/skia-telemetry/update_lua_task?key=$APPENGINE_KEY&lua_script_link=$SCRIPT_LINK&lua_output_link=$OUTPUT_LINK"

