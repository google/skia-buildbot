#!/bin/bash
#
# Starts the telemetry_slave_scripts/vm_run_pdf_viewer.sh script on all
# slaves.
#
# The script should be run from the skia-telemetry-master GCE instance's
# /home/default/skia-repo/buildbot/compute_engine_scripts/telemetry/telemetry_master_scripts
# directory.
#
# Copyright 2013 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)

if [ $# -ne 2 ]; then
  echo
  echo "Usage: `basename $0` rmistry@google.com rmistry-2013-05-24.07-34-05"
  echo
  echo "The first argument is the email address of the requester."
  echo "The second argument is a unique runid (typically requester + timestamp)."
  echo
  exit 1
fi

REQUESTER_EMAIL=$1
RUN_ID=$2

source ../vm_config.sh
source vm_utils.sh

# Update buildbot.
gclient sync

# Run vm_run_pdf_viewer.sh on all the slaves.
for SLAVE_NUM in $(seq 1 $NUM_SLAVES); do
  CMD="bash vm_run_pdf_viewer.sh $SLAVE_NUM $RUN_ID"
  ssh -f -X -o UserKnownHostsFile=/dev/null -o CheckHostIP=no \
    -o StrictHostKeyChecking=no -i /home/default/.ssh/google_compute_engine \
    -A -p 22 default@108.170.222.$SLAVE_NUM -- "source .bashrc; cd skia-repo/buildbot/compute_engine_scripts/telemetry/telemetry_slave_scripts; svn update; $CMD > /tmp/pdfviewer-${RUN_ID}_output.txt 2>&1"
done

# Sleep for a minute to give the slaves some time to start processing.
sleep 60

# Check to see if the slaves are done with this pdfviewer request.
SLAVES_STILL_PROCESSING=true
while $SLAVES_STILL_PROCESSING ; do
  SLAVES_STILL_PROCESSING=false
  for SLAVE_NUM in $(seq 1 $NUM_SLAVES); do
    RET=$( is_slave_currently_executing $SLAVE_NUM PDF_VIEWER.${RUN_ID} )
    if $RET; then
      echo "skia-telemetry-worker$SLAVE_NUM is still running PDF_VIEWER.${RUN_ID}"
      echo "Sleeping for a minute and then retrying"
      SLAVES_STILL_PROCESSING=true
      sleep 60
      break
    else
      echo "skia-telemetry-worker$SLAVE_NUM is done processing."
    fi
  done
done

# Copy over the outputs from all slaves and consolidate them into one file.
LOGS_DIR=/home/default/storage/pdf_logs
files=( "expected-skp.csv" "pdf-skp.csv" "actual-skp.csv" "csv-skp.csv" "csv-actual.csv" "result.csv" )
for SLAVE_NUM in $(seq 1 $NUM_SLAVES); do
  mkdir -p $LOGS_DIR/$RUN_ID/$SLAVE_NUM
  for file in "${files[@]}"; do
    gsutil cp gs://chromium-skia-gm/telemetry/pdfviewer/slave$SLAVE_NUM/outputs/${RUN_ID}/$file \
      $LOGS_DIR/$RUN_ID/$SLAVE_NUM/$file
    cat $LOGS_DIR/$RUN_ID/$SLAVE_NUM/$file >> $LOGS_DIR/$RUN_ID/$file
  done
done

# Copy the consolidated files into Google Storage.
for file in "${files[@]}"; do
  gsutil cp -a public-read $LOGS_DIR/$RUN_ID/$file \
    gs://chromium-skia-gm/telemetry/pdfviewer/consolidated-outputs/$RUN_ID/${file}.txt
done
OUTPUT_LINK=https://storage.cloud.google.com/chromium-skia-gm/telemetry/pdfviewer/consolidated-outputs/$RUN_ID

# Delete all tmp files.
# rm -rf $LOGS_DIR/*$RUN_ID*
# rm -rf /tmp/*$RUN_ID*

# Email the requester.
BOUNDARY=`date +%s|md5sum`
BOUNDARY=${BOUNDARY:0:32}
sendmail $REQUESTER_EMAIL <<EOF
subject:Your PDF Viewer task has completed!
to:$REQUESTER_EMAIL
from:skia.buildbot@gmail.com
Content-Type: multipart/mixed; boundary=\"$BOUNDARY\";

This is a MIME-encapsulated message

--$BOUNDARY
Content-Type: text/html

<html>
  <head/>
  <body>
  The output of your script is available <a href='$OUTPUT_LINK'>here</a>.<br/>
  Thanks!
  </body>
</html>

--$BOUNDARY--

EOF

