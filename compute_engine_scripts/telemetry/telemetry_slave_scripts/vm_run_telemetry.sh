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

if [ $# -ne 5 ]; then
  echo
  echo "Usage: `basename $0` 1 skpicture_printer alexa1-10000.json" \
    "--skp-outdir=/home/default/storage/skps/ rmistry"
  echo
  echo "The first argument is the slave_num of this telemetry slave."
  echo "The second argument is the telemetry benchmark to run on this slave."
  echo "The third argument is the page_set that should be processed."
  echo "The fourth argument are the extra arguments that the benchmark needs."
  echo "The fifth argument is the user who triggered the run."
  echo
  exit 1
fi

SLAVE_NUM=$1
TELEMETRY_BENCHMARK=$2
PAGE_SET_FILENAME=$3
EXTRA_ARGS=$4
REQUESTOR=$5

source vm_utils.sh

TIMESTAMP=`date "+%m-%d-%Y.%T"`
WORKER_FILE=$TELEMETRY_BENCHMARK.$REQUESTOR.$TIMESTAMP
create_worker_file $WORKER_FILE

source vm_setup_slave.sh

# Clean and create the skp output directory.
rm -rf /home/default/storage/skps
mkdir -p /home/default/storage/skps/

DISPLAY=:0 tools/perf/run_multipage_benchmarks --browser=system $TELEMETRY_BENCHMARK \
  /home/default/storage/page_sets/$PAGE_SET_FILENAME $EXTRA_ARGS

# Special handling for skpicture_printer, SKP files need to be copied to Google Storage.
if [ "$TELEMETRY_BENCHMARK" == "skpicture_printer" ]; then
  gsutil rm -R gs://chromium-skia-gm/telemetry/skps/slave$SLAVE_NUM/*
  cd /home/default/storage/skps
  SKP_LIST=`find . -mindepth 1 -maxdepth 1 -type d  \( ! -iname ".*" \) | sed 's|^\./||g'`
  for SKP in $SKP_LIST; do
    mv /home/default/storage/skps/$SKP/layer_0.skp /home/default/storage/skps/$SKP.skp
    gsutil cp /home/default/storage/skps/$SKP.skp \
      gs://chromium-skia-gm/telemetry/skps/slave$SLAVE_NUM/$SKP.skp
  done
  # Leave only SKP files in the skps directory.
  cd /home/default/storage/skps
  rm -rf */
fi

delete_worker_file $WORKER_FILE
