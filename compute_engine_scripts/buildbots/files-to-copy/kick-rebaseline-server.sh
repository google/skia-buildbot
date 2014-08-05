#!/bin/bash

# This file is copied to rebaseline_server machines and run there.
# See vm_setup_rebaseline_servers.sh

DEPOT_TOOLS=~/skia-repo/depot_tools
PIDFILE=~/rebaseline_server/pidfile
LOGS=~/rebaseline_server/logs
TRUNK=~/rebaseline_server/skia

cd ~/rebaseline_server

export PATH=$PATH:$DEPOT_TOOLS

if [ -f $PIDFILE ]; then
  PID=$(cat $PIDFILE)
  kill $PID
  rm $PIDFILE
fi
rm -f $LOGS

if [ ! -d $TRUNK ]; then
  $DEPOT_TOOLS/gclient config https://skia.googlesource.com/skia.git >>$LOGS 2>&1
fi
$DEPOT_TOOLS/gclient sync >>$LOGS 2>&1

# Build tools needed by rebaseline_server
pushd $TRUNK
make gyp >>$LOGS 2>&1
make tools BUILDTYPE=Release >>$LOGS 2>&1
popd $TRUNK

# Added --truncate test flag in here for now, so we can start exercising
# the new SKP results differ without killing the server with heavy load.
# TODO(rmistry): Once we move this to a beefier GCE instance, add "--threads 8"
$TRUNK/gm/rebaseline_server/server.py --port 10117 --export --reload 300 --truncate >>$LOGS 2>&1 &
echo $! >$PIDFILE
