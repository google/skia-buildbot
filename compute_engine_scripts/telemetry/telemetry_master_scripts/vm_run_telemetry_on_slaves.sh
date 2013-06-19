#!/bin/bash
#
# Starts the telemetry_slave_scripts/vm_run_telemetry.sh script on all
# slaves.
#
# The script should be run from the skia-telemetry-master GCE instance's
# /home/default/skia-repo/buildbot/compute_engine_scripts/telemetry/telemetry_master_scripts
# directory.
#
# Copyright 2013 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)

if [ $# -ne 6 ]; then
  echo
  echo "Usage: `basename $0` skpicture_printer" \
       "--skp-outdir=/home/default/storage/skps/ rmistry-2013-05-24.07-34-05" \
       "rmistry@google.com 1001 /tmp/logfile"
  echo
  echo "The first argument is the telemetry benchmark to run on this slave."
  echo "The second argument are the extra arguments that the benchmark needs."
  echo "The third argument is a unique runid (typically requester + timestamp)."
  echo "The fourth argument is the email address of the requester."
  echo "The fifth argument is the key of the appengine telemetry task."
  echo "The sixth argument is location of the log file."
  echo
  exit 1
fi

TELEMETRY_BENCHMARK=$1
EXTRA_ARGS=$2
RUN_ID=$3
REQUESTER_EMAIL=$4
APPENGINE_KEY=$5
LOG_FILE_LOCATION=$6

source ../vm_config.sh
source vm_utils.sh 

# Update buildbot.
gclient sync

# Check if any slave is in the process of capturing archives.
for SLAVE_NUM in $(seq 1 $NUM_SLAVES); do
  result=$(is_slave_currently_executing $SLAVE_NUM $RECORD_WPR_ACTIVITY)
  if $result; then
    echo
    echo "skia-telemetry-worker$SLAVE_NUM is currently capturing archives!"
    echo "Please rerun this script after it is done."
    echo
    exit 1
  fi
done

for SLAVE_NUM in $(seq 1 $NUM_SLAVES); do
  CMD="bash vm_run_telemetry.sh $SLAVE_NUM $TELEMETRY_BENCHMARK $EXTRA_ARGS $RUN_ID"
  ssh -f -X -o UserKnownHostsFile=/dev/null -o CheckHostIP=no \
    -o StrictHostKeyChecking=no -i /home/default/.ssh/google_compute_engine \
    -A -p 22 default@108.170.222.$SLAVE_NUM -- "source .bashrc; cd skia-repo/buildbot/compute_engine_scripts/telemetry/telemetry_slave_scripts; svn update; $CMD > /tmp/${TELEMETRY_BENCHMARK}-${RUN_ID}_output.txt 2>&1"
done

# Sleep for a minute to give the slaves some time to start processing.
sleep 60

# Check to see if the slaves are done with this telemetry benchmark.
SLAVES_STILL_PROCESSING=true
while $SLAVES_STILL_PROCESSING ; do
  SLAVES_STILL_PROCESSING=false
  for SLAVE_NUM in $(seq 1 $NUM_SLAVES); do
    RET=$( is_slave_currently_executing $SLAVE_NUM TELEMETRY_${RUN_ID} )
    if $RET; then
      echo "skia-telemetry-worker$SLAVE_NUM is still running TELEMETRY_${RUN_ID}"
      echo "Sleeping for a minute and then retrying"
      SLAVES_STILL_PROCESSING=true
      sleep 60
      break
    else
      echo "skia-telemetry-worker$SLAVE_NUM is done processing."
    fi
  done
done

# Email the requester.
BOUNDARY=`date +%s|md5sum`
BOUNDARY=${BOUNDARY:0:32}
sendmail $REQUESTER_EMAIL <<EOF
subject:Your Telemetry benchmark task has completed!
to:$REQUESTER_EMAIL
from:skia.buildbot@gmail.com
Content-Type: multipart/mixed; boundary=\"$BOUNDARY\";

This is a MIME-encapsulated message

--$BOUNDARY
Content-Type: text/html

<html>
  <head/>
  <body>
  You can schedule more runs <a href='https://skia-tree-status.appspot.com/skia-telemetry'>here</a>.<br/><br/>
  Thanks!
  </body>
</html>

--$BOUNDARY--

EOF

# Mark this task as completed on AppEngine.
PASSWORD=`cat appengine_password.txt`
wget --post-data "key=$APPENGINE_KEY&password=$PASSWORD" "https://skia-tree-status.appspot.com/skia-telemetry/update_telemetry_task" -O /dev/null

# Delete the log file since this task is now done.
rm $LOG_FILE_LOCATION
