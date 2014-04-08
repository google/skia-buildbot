#!/bin/bash
#
# Generates diff results for pdf viewer into a CSV file and copies it to Google
# Storage.
#
# The script should be run from the cluster-telemetry-slave GCE instance's
# /b/skia-repo/buildbot/cluster_telemetry/telemetry_slave_scripts
# directory.
#
# Copyright 2013 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)


if [ $# -ne 3 ]; then
  echo
  echo "Usage: `basename $0` 1 rmistry2013-05-24.07-34-05"
  echo
  echo "The first argument is the slave_num of this telemetry slave."
  echo "The second argument is the runid (typically requester + timestamp)."
  echo "The third argument is the type of pagesets to create from the 1M list" \
       "Eg: All, Filtered, 100k, 10k, Deeplinks."
  exit 1
fi

SLAVE_NUM=$1
RUN_ID=$2
PAGESETS_TYPE=$3

source vm_utils.sh

WORKER_FILE=PDF_VIEWER.$RUN_ID
create_worker_file $WORKER_FILE

TOOLS=`pwd`

# Sync trunk.
cd /b/skia-repo/trunk
/b/depot_tools/gclient sync

# Build tools, pdfviewer and skpdiff.
make tools BUILDTYPE=Release
./gyp_skia gyp/pdfviewer.gyp
make pdfviewer BUILDTYPE=Release

# Download the SKP files from Google Storage if the local TIMESTAMP is out of date.
mkdir -p /b/storage/skps/$PAGESETS_TYPE/
are_timestamps_equal /b/storage/skps/$PAGESETS_TYPE gs://chromium-skia-gm/telemetry/skps/slave$SLAVE_NUM/$PAGESETS_TYPE
if [ $? -eq 1 ]; then
  gsutil cp gs://chromium-skia-gm/telemetry/skps/slave$SLAVE_NUM/$PAGESETS_TYPE/* /b/storage/skps/$PAGESETS_TYPE/
fi

# Create directories for outputs of this script.
LOGS_DIR=/b/storage/pdf_logs/$RUN_ID
mkdir -p $LOGS_DIR/expected
mkdir -p $LOGS_DIR/pdf
mkdir -p $LOGS_DIR/actual
mkdir -p $LOGS_DIR/csv
mkdir -p $LOGS_DIR/result
mkdir -p $LOGS_DIR/logs

safe_tools=$(printf '%s\n' "$TOOLS" | sed 's/[\&/]/\\&/g')
safe_logs_dir=$(printf '%s\n' "$LOGS_DIR" | sed 's/[\&/]/\\&/g')

# Run render_pictures, render_pdfs, pdfviewer and skpdiff, in parallel - will use by all available cores
# Allow 5 minutes (300 seconds) for each skp to be proccessed.
ls /b/storage/skps/$PAGESETS_TYPE/*.skp | sed "s/^/${safe_tools}\/vm_timeout\.sh -t 300 ${safe_tools}\/vm_pdf_viewer_run_one_skp.sh ${safe_logs_dir} /" | parallel

# Merge all csv files in one result.csv file
cat $LOGS_DIR/csv/* | sort | uniq -u >$LOGS_DIR/result/result.csv

ls /b/storage/skps/$PAGESETS_TYPE/ | sed "s/\.skp//" >$LOGS_DIR/result/skp.csv
ls $LOGS_DIR/expected/ | sed "s/\.png//" >$LOGS_DIR/result/expected.csv
ls $LOGS_DIR/pdf/ | sed "s/\.pdf//" >$LOGS_DIR/result/pdf.csv
ls $LOGS_DIR/actual/ | sed "s/\.png//" >$LOGS_DIR/result/actual.csv
ls $LOGS_DIR/csv/ | sed "s/\.csv//" >$LOGS_DIR/result/csv.csv

# please upload these ones!
cat $LOGS_DIR/result/skp.csv $LOGS_DIR/result/expected.csv | sort | uniq -u >$LOGS_DIR/result/expected-skp.csv
cat $LOGS_DIR/result/skp.csv $LOGS_DIR/result/pdf.csv | sort | uniq -u >$LOGS_DIR/result/pdf-skp.csv
cat $LOGS_DIR/result/skp.csv $LOGS_DIR/result/actual.csv | sort | uniq -u >$LOGS_DIR/result/actual-skp.csv
cat $LOGS_DIR/result/skp.csv $LOGS_DIR/result/csv.csv | sort | uniq -u >$LOGS_DIR/result/csv-skp.csv
cat $LOGS_DIR/result/csv.csv $LOGS_DIR/result/actual.csv | sort | uniq -u >$LOGS_DIR/result/csv-actual.csv

# Copy the csv output and logs to Google Storage.
files=( "expected-skp.csv" "pdf-skp.csv" "actual-skp.csv" "csv-skp.csv" "csv-actual.csv" "result.csv" )
for file in "${files[@]}"; do
  gsutil cp $LOGS_DIR/result/$file gs://chromium-skia-gm/telemetry/pdfviewer/slave$SLAVE_NUM/outputs/${RUN_ID}/$file
done
gsutil cp /tmp/pdfviewer-${RUN_ID}_output.txt gs://chromium-skia-gm/telemetry/pdfviewer/slave$SLAVE_NUM/logs/${RUN_ID}.log

# Clean up logs and the worker file.
rm -rf /b/storage/pdf_logs/*${RUN_ID}*
rm -rf /tmp/*${RUN_ID}*
delete_worker_file $WORKER_FILE
