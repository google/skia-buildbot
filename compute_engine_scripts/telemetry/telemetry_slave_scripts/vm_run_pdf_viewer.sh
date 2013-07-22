#!/bin/bash
#
# Generates diff results for pdf viewer into a CSV file and copies it to Google
# Storage.
#
# The script should be run from the skia-telemetry-slave GCE instance's
# /home/default/skia-repo/buildbot/compute_engine_scripts/telemetry/telemetry_slave_scripts
# directory.
#
# Copyright 2013 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)


if [ $# -ne 2 ]; then
  echo
  echo "Usage: `basename $0` 1 rmistry2013-05-24.07-34-05"
  echo
  echo "The first argument is the slave_num of this telemetry slave."
  echo "The second argument is the runid (typically requester + timestamp)."
  echo
  exit 1
fi

SLAVE_NUM=$1
RUN_ID=$2

source vm_utils.sh

WORKER_FILE=PDF_VIEWER.$RUN_ID
create_worker_file $WORKER_FILE

# Sync trunk.
cd /home/default/skia-repo/trunk
/home/default/depot_tools/gclient sync

# Build tools, pdfviewer and skpdiff.
make tools BUILDTYPE=Release
./gyp_skia gyp/pdfviewer.gyp
make pdfviewer BUILDTYPE=Release
./gyp_skia experimental/skpdiff/skpdiff.gyp
make skpdiff BUILDTYPE=Release

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

# Create directories for outputs of this script.
LOGS_DIR=/home/default/storage/pdf_logs/$RUN_ID
mkdir -p $LOGS_DIR/expected
mkdir -p $LOGS_DIR/pdf
mkdir -p $LOGS_DIR/actual
mkdir -p $LOGS_DIR/csv

# Run render_pictures, render_pdfs, pdfviewer and skpdiff.
out/Release/render_pictures -r /home/default/storage/skps/ -w $LOGS_DIR/expected/
out/Release/render_pdfs /home/default/storage/skps/ -w $LOGS_DIR/pdf/
out/Release/pdfviewer -r $LOGS_DIR/pdf/ -w $LOGS_DIR/actual/ -n
out/Release/skpdiff -f $LOGS_DIR/expected/ $LOGS_DIR/actual/ --csv $LOGS_DIR/csv/result.csv

# Copy the csv output and logs to Google Storage.
gsutil cp $LOGS_DIR/csv/result.csv gs://chromium-skia-gm/telemetry/pdfviewer/slave$SLAVE_NUM/outputs/${RUN_ID}.output
gsutil cp /tmp/pdfviewer-${RUN_ID}_output.txt gs://chromium-skia-gm/telemetry/pdfviewer/slave$SLAVE_NUM/logs/${RUN_ID}.log

# Clean up logs and the worker file.
rm -rf /home/default/storage/pdf_logs/*${RUN_ID}*
rm -rf /tmp/*${RUN_ID}*
delete_worker_file $WORKER_FILE
