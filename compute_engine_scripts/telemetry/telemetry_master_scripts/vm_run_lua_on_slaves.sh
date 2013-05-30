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

if [ $# -ne 2 ]; then
  echo
  echo "Usage: `basename $0` gs://chromium-skia-gm/telemetry/lua_scripts/test.lua rmistry-2013-05-24.07-34-05"
  echo
  echo "The first argument is the Google Storage location of the Lua script."
  echo "The second argument is a unique run id (typically requester + timestamp)."
  echo
  exit 1
fi

LUA_SCRIPT_GS_LOCATION=$1
RUN_ID=$2

source ../vm_config.sh

# Update buildbot.
gclient sync

for SLAVE_NUM in $(seq 1 $NUM_SLAVES); do
  CMD="bash vm_run_lua_on_skps.sh $SLAVE_NUM $LUA_SCRIPT_GS_LOCATION $RUN_ID"
  ssh -f -X -o UserKnownHostsFile=/dev/null -o CheckHostIP=no \
    -o StrictHostKeyChecking=no -i /home/default/.ssh/google_compute_engine \
    -A -p 22 default@108.170.222.$SLAVE_NUM -- "source .bashrc; cd skia-repo/buildbot/compute_engine_scripts/telemetry/telemetry_slave_scripts; $CMD > /tmp/lua_output.txt 2>&1"
done
