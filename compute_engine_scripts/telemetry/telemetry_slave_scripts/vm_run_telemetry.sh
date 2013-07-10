#!/bin/bash
#
# Runs all steps in vm_setup_slave.sh, calls run_multipage_benchmarks and copies
# the output (if any) to Google Storage.
#
# The script should be run from the skia-telemetry-slave GCE instance's
# /home/default/skia-repo/buildbot/compute_engine_scripts/telemetry/telemetry_slave_scripts
# directory.
#
# Copyright 2013 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)


if [ $# -lt 4 ]; then
  echo
  echo "Usage: `basename $0` 1 skpicture_printer" \
    "--skp-outdir=/home/default/storage/skps/ rmistry2013-05-24.07-34-05"
  echo
  echo "The first argument is the slave_num of this telemetry slave."
  echo "The second argument is the telemetry benchmark to run on this slave."
  echo "The third argument are the extra arguments that the benchmark needs."
  echo "The fourth argument is the runid (typically requester + timestamp)."
  echo "The fifth optional argument is the Google Storage location of the URL whitelist."
  echo
  exit 1
fi

SLAVE_NUM=$1
TELEMETRY_BENCHMARK=$2
EXTRA_ARGS=$3
RUN_ID=$4
WHITELIST_GS_LOCATION=$5

WHITELIST_FILE=whitelist.$RUN_ID

source vm_utils.sh

create_worker_file TELEMETRY_${RUN_ID}

source vm_setup_slave.sh

# Download the webpage_archives from Google Storage if the local TIMESTAMP is
# out of date.
mkdir -p /home/default/storage/webpages_archive/
are_timestamps_equal /home/default/storage/webpages_archive gs://chromium-skia-gm/telemetry/webpages_archive/slave$SLAVE_NUM
if [ $? -eq 1 ]; then
  gsutil cp gs://chromium-skia-gm/telemetry/webpages_archive/slave$SLAVE_NUM/* \
    /home/default/storage/webpages_archive/
fi

if [[ ! -z "$WHITELIST_GS_LOCATION" ]]; then
  # Copy the whitelist from Google Storage to /tmp.
  gsutil cp $WHITELIST_GS_LOCATION /tmp/$WHITELIST_FILE
fi

if [ "$TELEMETRY_BENCHMARK" == "skpicture_printer" ]; then
  # Clean and create the skp output directory.
  rm -rf /home/default/storage/skps
  mkdir -p /home/default/storage/skps/
fi

for page_set in /home/default/storage/page_sets/*.json; do
  if [[ -f $page_set ]]; then
    if [[ ! -z "$WHITELIST_GS_LOCATION" ]]; then
      check_pageset_url_in_whitelist $page_set /tmp/$WHITELIST_FILE
      if [ $? -eq 1 ]; then
        # The current page set URL does not exist in the whitelist, move on to
        # the next one.
        echo "========== Skipping $page_set because it is not in the whitelist =========="
        continue
      fi
    fi
    echo "========== Processing $page_set =========="
    page_set_basename=`basename $page_set`
    DISPLAY=:0 timeout 600 tools/perf/run_measurement --extra-browser-args=--disable-setuid-sandbox --browser=system $TELEMETRY_BENCHMARK $page_set $EXTRA_ARGS -o /tmp/${RUN_ID}.${page_set_basename}
    if [ $? -eq 124 ]; then
      echo "========== $page_set timed out! =========="
    else
      echo "========== Done with $page_set =========="
    fi
  fi
done

# Consolidate outputs from all page sets into a single file with special
# handling for CSV files.
for output in /tmp/${RUN_ID}.*; do
  if [[ "$EXTRA_ARGS" == *--output-format=csv* ]]; then
    # For CSV outputs, save the first encountered CSV and append only the last
    # line of all other CSVs. This ensures that the headings appear only once.
    if [ ! -f /tmp/output.${RUN_ID} ]; then
      cp $output /tmp/output.${RUN_ID}
    else
      tail -n -1 "$output" >> /tmp/output.${RUN_ID}
    fi
  else
    cat $output >> /tmp/output.${RUN_ID}
  fi
done

# Copy the consolidated output to Google Storage.
gsutil cp /tmp/output.${RUN_ID} gs://chromium-skia-gm/telemetry/benchmarks/$TELEMETRY_BENCHMARK/slave$SLAVE_NUM/outputs/${RUN_ID}.output
# Copy the complete telemetry log to Google Storage.
gsutil cp /tmp/${TELEMETRY_BENCHMARK}-${RUN_ID}_output.txt gs://chromium-skia-gm/telemetry/benchmarks/$TELEMETRY_BENCHMARK/slave$SLAVE_NUM/logs/${RUN_ID}.log

# Special handling for skpicture_printer, SKP files need to be copied to Google Storage.
if [ "$TELEMETRY_BENCHMARK" == "skpicture_printer" ]; then
  gsutil rm -R gs://chromium-skia-gm/telemetry/skps/slave$SLAVE_NUM/*
  cd /home/default/storage/skps
  SKP_LIST=`find . -mindepth 1 -maxdepth 1 -type d  \( ! -iname ".*" \) | sed 's|^\./||g'`
  for SKP in $SKP_LIST; do
    mv /home/default/storage/skps/$SKP/layer_0.skp /home/default/storage/skps/$SKP.skp
  done

  # Leave only SKP files in the skps directory.
  rm -rf /home/default/storage/skps/*/

  # Delete all SKP files less than 10K (they will be the ones with errors).
  find . -type f -size -10k
  find . -type f -size -10k -exec rm {} \;

  # Now copy the SKP files to Google Storage. 
  gsutil cp /home/default/storage/skps/* \
    gs://chromium-skia-gm/telemetry/skps/slave$SLAVE_NUM/

  # Create a TIMESTAMP file and copy it to Google Storage.
  TIMESTAMP=`date +%s`
  echo $TIMESTAMP > /tmp/$TIMESTAMP
  cp /tmp/$TIMESTAMP /home/default/storage/skps/
  gsutil cp /tmp/$TIMESTAMP gs://chromium-skia-gm/telemetry/skps/slave$SLAVE_NUM/TIMESTAMP
  rm /tmp/$TIMESTAMP
fi

# Clean up logs and the worker file.
rm -rf /tmp/*${RUN_ID}*
delete_worker_file TELEMETRY_${RUN_ID}
