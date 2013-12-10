#!/bin/bash
#
# Starts the telemetry_slave_scripts/vm_run_telemetry.sh script on all
# slaves.
# Use TRYSERVER=true to skip uploading files to Google Storage or emailing
# the requester.
#
# The script should be run from the skia-telemetry-master GCE instance's
# /home/default/skia-repo/buildbot/compute_engine_scripts/telemetry/telemetry_master_scripts
# directory.
#
# Copyright 2013 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)

if [ $# -lt 5 ]; then
  echo
  echo "Usage: `basename $0` skpicture_printer" \
       "--skp-outdir=/home/default/storage/skps/ All" \
       "a1234b-c5678d rmistry-2013-05-24.07-34-05" \
       "rmistry@google.com 1001 /tmp/logfile"
  echo
  echo "The first argument is the telemetry benchmark to run on this slave."
  echo "The second argument are the extra arguments that the benchmark needs."
  echo "The third argument is the type of pagesets to create from the 1M list" \
       "Eg: All, Filtered, 100k, 10k, Deeplinks."
  echo "The fourth argument is the name of the directory where the chromium" \
       "build which will be used for this run is stored."
  echo "The fifth argument is a unique runid (typically requester + timestamp)."
  echo "The sixth argument is the email address of the requester (optional)."
  echo "The seventh argument is the key of the appengine telemetry task (optional)."
  echo "The eighth argument is the location of the log file (optional)."
  echo "The ninth argument is the local location of the optional whitelist file (optional)."
  echo
  exit 1
fi

TELEMETRY_BENCHMARK=$1
EXTRA_ARGS=$2
PAGESETS_TYPE=$3
CHROMIUM_BUILD_DIR=$4
RUN_ID=$5
REQUESTER_EMAIL=$6
APPENGINE_KEY=$7
LOG_FILE_LOCATION=$8
WHITELIST_LOCAL_LOCATION=$9

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

if [[ ! -z "$WHITELIST_LOCAL_LOCATION" ]]; then
  # Copy the whitelist to Google Storage.
  WHITELIST_GS_LOCATION=gs://chromium-skia-gm/telemetry/benchmarks/$TELEMETRY_BENCHMARK/whitelists/$RUN_ID.whitelist
  gsutil cp -a public-read $WHITELIST_LOCAL_LOCATION $WHITELIST_GS_LOCATION
fi


for SLAVE_NUM in $(seq 1 $NUM_SLAVES); do
  CMD="bash vm_run_telemetry.sh $SLAVE_NUM $TELEMETRY_BENCHMARK \"$EXTRA_ARGS\" $PAGESETS_TYPE $CHROMIUM_BUILD_DIR $RUN_ID $WHITELIST_GS_LOCATION"
  ssh -f -X -o UserKnownHostsFile=/dev/null -o CheckHostIP=no \
    -o StrictHostKeyChecking=no -i /home/default/.ssh/google_compute_engine \
    -A -p 22 default@108.170.222.$SLAVE_NUM -- "source .bashrc; cd skia-repo/buildbot/compute_engine_scripts/telemetry/telemetry_slave_scripts; /home/default/depot_tools/gclient sync; $CMD > /tmp/${TELEMETRY_BENCHMARK}-${RUN_ID}_output.txt 2>&1"
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

# Copy over the outputs from all slaves and consolidate them into one file with
# special handling for CSV files.
OUTPUT_DIR=/home/default/storage/telemetry_outputs/$RUN_ID
mkdir -p $OUTPUT_DIR
for SLAVE_NUM in $(seq 1 $NUM_SLAVES); do
  gsutil cp gs://chromium-skia-gm/telemetry/benchmarks/$TELEMETRY_BENCHMARK/slave$SLAVE_NUM/outputs/$RUN_ID.output \
    $OUTPUT_DIR/$SLAVE_NUM.output
  if [[ "$EXTRA_ARGS" == *--output-format=csv* ]]; then
    mv $OUTPUT_DIR/$SLAVE_NUM.output $OUTPUT_DIR/$SLAVE_NUM.csv
  else
    cat $OUTPUT_DIR/$SLAVE_NUM.output >> $OUTPUT_DIR/${RUN_ID}.$TELEMETRY_BENCHMARK.output
  fi
done

if [[ "$EXTRA_ARGS" == *--output-format=csv* ]]; then
  python ../csv_merger.py --csv_dir=$OUTPUT_DIR --output_csv_name=${RUN_ID}.$TELEMETRY_BENCHMARK.output
fi

# Delete all tmp files from the slaves because telemetry may have generated a
# lot of temporary artifacts there and they take up root disk space.
bash vm_run_command_on_slaves.sh "cd /tmp/; find . -maxdepth 1 -type d -name '*' -exec sudo rm -rf {} \\;"
bash vm_run_command_on_slaves.sh "cd /tmp/; find . -maxdepth 1 -type f -name '*' -exec sudo rm -rf {} \\;"

# Copy the consolidated file into Google Storage.
gsutil cp -a public-read $OUTPUT_DIR/$RUN_ID.$TELEMETRY_BENCHMARK.output \
    gs://chromium-skia-gm/telemetry/benchmarks/$TELEMETRY_BENCHMARK/consolidated-outputs/$RUN_ID.output.txt
# Setting ACLs on massive files sometimes does not work so do it explicitly.
gsutil setacl public-read gs://chromium-skia-gm/telemetry/benchmarks/$TELEMETRY_BENCHMARK/consolidated-outputs/$RUN_ID.output.txt

if [ ! -n "$TRYSERVER" ]; then
  OUTPUT_LINK=https://storage.cloud.google.com/chromium-skia-gm/telemetry/benchmarks/$TELEMETRY_BENCHMARK/consolidated-outputs/$RUN_ID.output.txt
  SLAVE_1_LOG_LINK=https://storage.cloud.google.com/chromium-skia-gm/telemetry/benchmarks/$TELEMETRY_BENCHMARK/slave1/logs/$RUN_ID.log

  # Delete all tmp files.
  rm -rf /tmp/$RUN_ID*
  rm -rf ${OUTPUT_DIR}*

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
  The output of your script is available <a href='$OUTPUT_LINK'>here</a>.<br/><br/>
  If the above output is blank and you expected output then please look for failures in the log file of the first slave <a href='$SLAVE_1_LOG_LINK'>here</a>.<br/>
  You can schedule more runs <a href='https://skia-tree-status.appspot.com/skia-telemetry'>here</a>.<br/><br/>
  Thanks!
  </body>
</html>

--$BOUNDARY--

EOF

  # Mark this task as completed on AppEngine.
  PASSWORD=`cat /home/default/skia-repo/buildbot/compute_engine_scripts/telemetry/telemetry_master_scripts/appengine_password.txt`
  wget --post-data "key=$APPENGINE_KEY&output_link=$OUTPUT_LINK&password=$PASSWORD" "https://skia-tree-status.appspot.com/skia-telemetry/update_telemetry_task" -O /dev/null
fi

# Delete the log file since this task is now done.
rm $LOG_FILE_LOCATION
