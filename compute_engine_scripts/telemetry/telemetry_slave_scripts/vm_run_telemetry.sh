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


if [ $# -ne 4 ]; then
  echo
  echo "Usage: `basename $0` 1 skpicture_printer" \
    "--skp-outdir=/home/default/storage/skps/ rmistry"
  echo
  echo "The first argument is the slave_num of this telemetry slave."
  echo "The second argument is the telemetry benchmark to run on this slave."
  echo "The third argument are the extra arguments that the benchmark needs."
  echo "The fourth argument is the user who triggered the run."
  echo
  exit 1
fi

SLAVE_NUM=$1
TELEMETRY_BENCHMARK=$2
EXTRA_ARGS=$3
REQUESTOR=$4

source vm_utils.sh

TIMESTAMP=`date "+%m-%d-%Y.%T"`
WORKER_FILE=$TELEMETRY_BENCHMARK.$REQUESTOR.$TIMESTAMP
create_worker_file $WORKER_FILE

source vm_setup_slave.sh

# Download webpage_archives from Google Storage.
mkdir -p /home/default/storage/webpages_archive/
gsutil cp gs://chromium-skia-gm/telemetry/webpages_archive/slave$SLAVE_NUM/* \
  /home/default/storage/webpages_archive/

# Download page_sets from Google Storage.
mkdir -p /home/default/storage/page_sets/
rm -rf /home/default/storage/page_sets/*
gsutil cp gs://chromium-skia-gm/telemetry/page_sets/slave$SLAVE_NUM/* \
  /home/default/storage/page_sets/

# Clean and create the skp output directory.
rm -rf /home/default/storage/skps
mkdir -p /home/default/storage/skps/

for page_set in /home/default/storage/page_sets/*; do
  if [[ -f $page_set ]]; then
    echo "========== Processing $page_set =========="
    DISPLAY=:0 tools/perf/run_multipage_benchmarks --browser=system $TELEMETRY_BENCHMARK $page_set $EXTRA_ARGS
    echo "========== Done with $page_set =========="
  fi
done

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
fi

delete_worker_file $WORKER_FILE
