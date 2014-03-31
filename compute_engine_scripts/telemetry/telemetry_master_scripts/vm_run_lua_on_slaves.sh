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

if [ $# -lt 6 ]; then
  echo
  echo "Usage: `basename $0` /tmp/test.lua" \
       "rmistry-2013-05-24.07-34-05 rmistry@google.com 1001"
  echo
  echo "The first argument is the local location of the Lua script."
  echo "The second argument is a unique run id (typically requester + timestamp)."
  echo "The third argument is the type of pagesets to create from the 1M list" \
       "Eg: All, Filtered, 100k, 10k, Deeplinks."
  echo "The fourth argument is the name of the directory where the chromium" \
       "build which will be used for this run is stored."
  echo "The fifth argument is the email address of the requester."
  echo "The sixth argument is the key of the appengine lua task."
  echo "The seventh argument is the local location of the Lua aggregator script.(optional)"
  echo
  exit 1
fi

LUA_SCRIPT_LOCAL_LOCATION=$1
RUN_ID=$2
PAGESETS_TYPE=$3
CHROMIUM_BUILD_DIR=$4
REQUESTER_EMAIL=$5
APPENGINE_KEY=$6
LUA_AGGREGATOR_LOCAL_LOCATION=$7

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

LOGS_DIR=/home/default/storage/lua_logs
mkdir $LOGS_DIR

# Run vm_run_lua_on_skps.sh on all the slaves.
for SLAVE_NUM in $(seq 1 $NUM_SLAVES); do
  CMD="bash vm_run_lua_on_skps.sh $SLAVE_NUM $LUA_SCRIPT_GS_LOCATION $RUN_ID $PAGESETS_TYPE $CHROMIUM_BUILD_DIR"
  ssh -f -X -o UserKnownHostsFile=/dev/null -o CheckHostIP=no \
    -o StrictHostKeyChecking=no -i /home/default/.ssh/google_compute_engine \
    -A -p 22 default@108.170.192.$SLAVE_NUM -- "source .bashrc; cd skia-repo/buildbot/compute_engine_scripts/telemetry/telemetry_slave_scripts; $CMD > /tmp/lua_output.$RUN_ID.txt 2>&1"
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
    $LOGS_DIR/$RUN_ID-$SLAVE_NUM.lua-output
  cat $LOGS_DIR/$RUN_ID-$SLAVE_NUM.lua-output >> $LOGS_DIR/$RUN_ID.lua-output
done

# Run the aggregator if specified.
if [ -n "$LUA_AGGREGATOR_LOCAL_LOCATION" ]; then
  cp $LOGS_DIR/$RUN_ID.lua-output /tmp/lua-output
  lua $LUA_AGGREGATOR_LOCAL_LOCATION &> $LOGS_DIR/$RUN_ID.lua-output

  # Copy the original slave output to Google Storage.
  gsutil cp -a public-read /tmp/lua-output \
    gs://chromium-skia-gm/telemetry/lua-outputs/consolidated-outputs/$RUN_ID.original-output.txt
  ORIGINAL_OUTPUT_LINK=https://storage.cloud.google.com/chromium-skia-gm/telemetry/lua-outputs/consolidated-outputs/$RUN_ID.original-output.txt
  ORIGINAL_OUTPUT_TXT="The pre-aggregated output is available <a href='$ORIGINAL_OUTPUT_LINK'>here</a>.<br/><br/>"
  rm /tmp/lua-output

  # Copy the aggregator file into Google Storage.
  gsutil cp -a public-read $LUA_AGGREGATOR_LOCAL_LOCATION \
    gs://chromium-skia-gm/telemetry/lua-outputs/consolidated-outputs/$RUN_ID.aggregator.txt
  AGGREGATOR_LINK=https://storage.cloud.google.com/chromium-skia-gm/telemetry/lua-outputs/consolidated-outputs/$RUN_ID.aggregator.txt
fi

# Copy the consolidated file into Google Storage.
gsutil cp -a public-read $LOGS_DIR/$RUN_ID.lua-output \
  gs://chromium-skia-gm/telemetry/lua-outputs/consolidated-outputs/$RUN_ID.lua-output.txt

# Delete all tmp files.
rm -rf $LOGS_DIR/$RUN_ID*
rm -rf /tmp/$RUN_ID*

# End the timer.
TIMER="$(($(date +%s)-TIMER))"

# Email results to the requester and admins (should be comma separated).
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
  $ORIGINAL_OUTPUT_TXT
  You can schedule more runs <a href='https://skia-tree-status.appspot.com/skia-telemetry/lua_script'>here</a>.<br/><br/>
  Thanks!
  </body>
</html>

--$BOUNDARY--

EOF

# Mark this task as completed on AppEngine.
PASSWORD=`cat /home/default/skia-repo/buildbot/compute_engine_scripts/telemetry/telemetry_master_scripts/appengine_password.txt`
wget --post-data "key=$APPENGINE_KEY&lua_script_link=$SCRIPT_LINK&lua_output_link=$OUTPUT_LINK&lua_aggregator_link=$AGGREGATOR_LINK&password=$PASSWORD" "https://skia-tree-status.appspot.com/skia-telemetry/update_lua_task" -O /dev/null

