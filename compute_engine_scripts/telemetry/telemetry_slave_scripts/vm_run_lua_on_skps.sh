#!/bin/bash
#
# Runs a Lua script from Google Storage on the SKP files on this slave.
#
# The script should be run from the skia-telemetry-slave GCE instance's
# /home/default/skia-repo/buildbot/compute_engine_scripts/telemetry/telemetry_slave_scripts
# directory.
#
# Copyright 2013 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)

if [ $# -ne 3 ]; then
  echo
  echo "Usage: `basename $0` 1 gs://chromium-skia-gm/telemetry/lua_scripts/test.lua" \
    "rmistry-2013-05-24.07-34-05"
  echo
  echo "The first argument is the slave_num of this telemetry slave."
  echo "The second argument is the Google Storage location of the Lua script."
  echo "The third argument is a unique run id (typically requester + timestamp)."
  echo
  exit 1
fi

SLAVE_NUM=$1
LUA_SCRIPT_GS_LOCATION=$2
RUN_ID=$3

source vm_utils.sh

WORKER_FILE=LUA.$RUN_ID
LUA_FILE=$RUN_ID.lua
LUA_OUTPUT_FILE=$RUN_ID.lua-output
create_worker_file $WORKER_FILE

# Sync trunk.
cd /home/default/skia-repo/trunk
/home/default/depot_tools/gclient sync

# Build tools.
# make clean
GYP_DEFINES="skia_warnings_as_errors=0" make tools BUILDTYPE=Release

if [ -e /etc/boto.cfg ]; then
  # Move boto.cfg since it may interfere with the ~/.boto file.
  sudo mv /etc/boto.cfg /etc/boto.cfg.bak
fi

# Download the SKP files from Google Storage if the local TIMESTAMP is out of date.
mkdir -p /home/default/storage/skps/
are_timestamps_equal /home/default/storage/skps gs://chromium-skia-gm/telemetry/skps/slave$SLAVE_NUM
if [ $? -eq 1 ]; then
  gsutil cp gs://chromium-skia-gm/telemetry/skps/slave$SLAVE_NUM/* /home/default/storage/skps/
fi

# Copy the lua script from Google Storage to /tmp.
gsutil cp $LUA_SCRIPT_GS_LOCATION /tmp/$LUA_FILE

# Run lua_pictures.
cd out/Release
./lua_pictures --skpPath /home/default/storage/skps/ --luaFile /tmp/$LUA_FILE > /tmp/$LUA_OUTPUT_FILE

# Copy the output of the lua script to Google Storage.
gsutil cp /tmp/$LUA_OUTPUT_FILE gs://chromium-skia-gm/telemetry/lua-outputs/slave$SLAVE_NUM/$LUA_OUTPUT_FILE

# Clean up logs and the worker file.
rm -rf /tmp/*${RUN_ID}*
delete_worker_file $WORKER_FILE
