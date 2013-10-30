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


if [ $# -lt 5 ]; then
  echo
  echo "Usage: `basename $0` 1 skpicture_printer" \
    "--skp-outdir=/home/default/storage/skps/ All a1234b-c5678d" \
    "rmistry2013-05-24.07-34-05"
  echo
  echo "The first argument is the slave_num of this telemetry slave."
  echo "The second argument is the telemetry benchmark to run on this slave."
  echo "The third argument are the extra arguments that the benchmark needs."
  echo "The fourth argument is the type of pagesets to create from the 1M list" \
       "Eg: All, Filtered, 100k, 10k, Deeplinks."
  echo "The fifth argument is the name of the directory where the chromium" \
       "build which will be used for this run is stored."
  echo "The sixth argument is the runid (typically requester + timestamp)."
  echo "The seventh optional argument is the Google Storage location of the URL" \
       "whitelist."
  echo
  exit 1
fi

SLAVE_NUM=$1
TELEMETRY_BENCHMARK=$2
EXTRA_ARGS=$3
PAGESETS_TYPE=$4
CHROMIUM_BUILD_DIR=$5
RUN_ID=$6
WHITELIST_GS_LOCATION=$7

WHITELIST_FILE=whitelist.$RUN_ID

source vm_utils.sh

create_worker_file TELEMETRY_${RUN_ID}

source vm_setup_slave.sh

# Download the webpage_archives from Google Storage if the local TIMESTAMP is
# out of date.
mkdir -p /home/default/storage/webpages_archive/$PAGESETS_TYPE/
are_timestamps_equal /home/default/storage/webpages_archive/$PAGESETS_TYPE gs://chromium-skia-gm/telemetry/webpages_archive/slave$SLAVE_NUM/$PAGESETS_TYPE
if [ $? -eq 1 ]; then
  gsutil cp gs://chromium-skia-gm/telemetry/webpages_archive/slave$SLAVE_NUM/$PAGESETS_TYPE/* \
    /home/default/storage/webpages_archive/$PAGESETS_TYPE
fi

if [[ ! -z "$WHITELIST_GS_LOCATION" ]]; then
  # Copy the whitelist from Google Storage to /tmp.
  gsutil cp $WHITELIST_GS_LOCATION /tmp/$WHITELIST_FILE
fi

if [ "$TELEMETRY_BENCHMARK" == "skpicture_printer" ]; then
  # Clean and create the skp output directory.
  sudo chown -R default:default /home/default/storage/skps/$PAGESETS_TYPE
  rm -rf /home/default/storage/skps/$PAGESETS_TYPE
  mkdir -p /home/default/storage/skps/$PAGESETS_TYPE/
fi

OUTPUT_DIR=/home/default/storage/telemetry_outputs/$RUN_ID
mkdir -p $OUTPUT_DIR

for page_set in /home/default/storage/page_sets/$PAGESETS_TYPE/*.json; do
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
    check_and_run_xvfb
    eval sudo DISPLAY=:0 timeout 300 tools/perf/run_measurement --extra-browser-args=\"--disable-setuid-sandbox --enable-software-compositing\" --browser-executable=/home/default/storage/chromium-builds/${CHROMIUM_BUILD_DIR}/chrome --browser=exact $TELEMETRY_BENCHMARK $page_set $EXTRA_ARGS -o $OUTPUT_DIR/${RUN_ID}.${page_set_basename}
    sudo chown default:default $OUTPUT_DIR/${RUN_ID}.${page_set_basename}
    if [ $? -eq 124 ]; then
      echo "========== $page_set timed out! =========="
    else
      echo "========== Done with $page_set =========="
    fi
  fi
done

# Consolidate outputs from all page sets into a single file with special
# handling for CSV files.
mkdir $OUTPUT_DIR/${RUN_ID}

for output in $OUTPUT_DIR/${RUN_ID}.*; do
  if [[ "$EXTRA_ARGS" == *--output-format=csv* ]]; then
    csv_basename=`basename $output`
    mv $output $OUTPUT_DIR/${RUN_ID}/${csv_basename}.csv
  else
    cat $output >> $OUTPUT_DIR/output.${RUN_ID}
  fi
done

if [[ "$EXTRA_ARGS" == *--output-format=csv* ]]; then
  python ~/skia-repo/buildbot/compute_engine_scripts/telemetry/csv_merger.py \
    --csv_dir=$OUTPUT_DIR/${RUN_ID} --output_csv_name=$OUTPUT_DIR/output.${RUN_ID}
fi

# Copy the consolidated output to Google Storage.
gsutil cp $OUTPUT_DIR/output.${RUN_ID} gs://chromium-skia-gm/telemetry/benchmarks/$TELEMETRY_BENCHMARK/slave$SLAVE_NUM/outputs/${RUN_ID}.output
# Copy the complete telemetry log to Google Storage.
gsutil cp /tmp/${TELEMETRY_BENCHMARK}-${RUN_ID}_output.txt gs://chromium-skia-gm/telemetry/benchmarks/$TELEMETRY_BENCHMARK/slave$SLAVE_NUM/logs/${RUN_ID}.log

# Special handling for skpicture_printer, SKP files need to be copied to Google Storage.
if [ "$TELEMETRY_BENCHMARK" == "skpicture_printer" ]; then
  gsutil rm -R gs://chromium-skia-gm/telemetry/skps/slave$SLAVE_NUM/$PAGESETS_TYPE/*
  sudo chown -R default:default /home/default/storage/skps/$PAGESETS_TYPE
  cd /home/default/storage/skps/$PAGESETS_TYPE
  SKP_LIST=`find . -mindepth 1 -maxdepth 1 -type d  \( ! -iname ".*" \) | sed 's|^\./||g'`
  for SKP in $SKP_LIST; do
    mv /home/default/storage/skps/$PAGESETS_TYPE/$SKP/layer_0.skp /home/default/storage/skps/$PAGESETS_TYPE/$SKP.skp
  done

  # Leave only SKP files in the skps directory.
  rm -rf /home/default/storage/skps/$PAGESETS_TYPE/*/

  # Delete all SKP files less than 10K (they will be the ones with errors).
  find . -type f -size -10k
  find . -type f -size -10k -exec rm {} \;

  # Now copy the SKP files to Google Storage. 
  gsutil cp /home/default/storage/skps/$PAGESETS_TYPE/* \
    gs://chromium-skia-gm/telemetry/skps/slave$SLAVE_NUM/$PAGESETS_TYPE/

  # Create a TIMESTAMP file and copy it to Google Storage.
  TIMESTAMP=`date +%s`
  echo $TIMESTAMP > /tmp/$TIMESTAMP
  cp /tmp/$TIMESTAMP /home/default/storage/skps/$PAGESETS_TYPE/TIMESTAMP
  gsutil cp /tmp/$TIMESTAMP gs://chromium-skia-gm/telemetry/skps/slave$SLAVE_NUM/$PAGESETS_TYPE/TIMESTAMP
  rm /tmp/$TIMESTAMP
fi

# Clean up logs and the worker file.
rm -rf /tmp/*${RUN_ID}*
rm -rf ${OUTPUT_DIR}*
delete_worker_file TELEMETRY_${RUN_ID}
