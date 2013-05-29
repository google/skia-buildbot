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
  echo "Usage: `basename $0` 1 skpicture_printer alexa1-10000.json" \
    " --skp-outdir=/home/default/storage/skps/"
  exit 1
fi

SLAVE_NUM=$1
TELEMETRY_BENCHMARK=$2
PAGE_SET_FILENAME=$3
EXTRA_ARGS=$4

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
    gsutil cp /home/default/storage/skps/$SKP/layer_0.skp \
      gs://chromium-skia-gm/telemetry/skps/slave$SLAVE_NUM/$SKP.skp
  done
fi

