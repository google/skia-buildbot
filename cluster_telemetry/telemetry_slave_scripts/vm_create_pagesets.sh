#!/bin/bash
#
# Create pagesets for this slave.
#
# The script should be run from the cluster-telemetry-slave GCE instance's
# /b/skia-repo/buildbot/cluster_telemetry/telemetry_slave_scripts
# directory.
#
# Copyright 2013 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)

if [ $# -ne 3 ]; then
  echo
  echo "Usage: `basename $0` 1 1 All"
  echo
  echo "The first argument is the slave_num of this telemetry slave."
  echo "The second argument is the webpage rank to start with."
  echo "The third argument is the type of pagesets to create from the 1M list" \
       "Eg: All, Filtered, 100k, 10k, Deeplinks."
  echo
  exit 1
fi

SLAVE_NUM=$1
WEBPAGES_START=$2
PAGESETS_TYPE=$3

source vm_utils.sh
source ../config.sh

create_worker_file $CREATING_PAGESETS_ACTIVITY

# Sync buildbot.
/b/depot_tools/gclient sync

# Move into the buildbot/tools directory.
cd ../../tools
# Delete the old page_sets.
rm -rf page_sets/*.py
# Clean and create directories where page_sets will be stored.
rm -rf /b/storage/page_sets/$PAGESETS_TYPE/*

# If PAGESETS_TYPE is 10k or Mobile10k then adjust NUM_WEBPAGES.
if [ "$PAGESETS_TYPE" == "10k" ] || [ "$PAGESETS_TYPE" == "Mobile10k" ]; then
  NUM_WEBPAGES=10000
fi

NUM_WEBPAGES_PER_SLAVE=$(($NUM_WEBPAGES/$NUM_SLAVES))
NUM_PAGESETS_PER_SLAVE=$(($NUM_WEBPAGES_PER_SLAVE/$MAX_WEBPAGES_PER_PAGESET))
START=$WEBPAGES_START

if [ "$PAGESETS_TYPE" == "Mobile10k" ]; then
  CSV_PATH="page_sets/android-top-1m.csv"
  # Download the mobile 10k from Google Storage.
  gsutil cp gs://chromium-skia-gm/telemetry/csv/android-top-1m.csv $CSV_PATH
  # Use a mobile useragent.
  USERAGENT="mobile"
else
  CSV_PATH="page_sets/top-1m.csv"
  # Download the desktop 10k/1M from Google Storage.
  gsutil cp gs://chromium-skia-gm/telemetry/csv/top-1m.csv $CSV_PATH
  USERAGENT="desktop"
fi

# Run create_page_set.py
for PAGESET_NUM in $(seq 1 $NUM_PAGESETS_PER_SLAVE); do
  END=$(expr $START + $MAX_WEBPAGES_PER_PAGESET - 1)
  python create_page_set.py -s $START -e $END \
    -c $CSV_PATH -p $PAGESETS_TYPE -u $USERAGENT
  START=$(expr $END + 1)
done
# Copy page_sets to the local directory.
mkdir -p /b/storage/page_sets/$PAGESETS_TYPE
mv page_sets/*.py /b/storage/page_sets/$PAGESETS_TYPE/

# Clean the directory in Google Storage.
gsutil rm -R gs://chromium-skia-gm/telemetry/page_sets/slave$SLAVE_NUM/$PAGESETS_TYPE/*
# Copy the page_sets into Google Storage.
gsutil -m cp /b/storage/page_sets/$PAGESETS_TYPE/*py gs://chromium-skia-gm/telemetry/page_sets/slave$SLAVE_NUM/$PAGESETS_TYPE/

# Create a TIMESTAMP file and copy it to Google Storage.
TIMESTAMP=`date +%s`
echo $TIMESTAMP > /tmp/$TIMESTAMP
cp /tmp/$TIMESTAMP /b/storage/page_sets/$PAGESETS_TYPE/TIMESTAMP
gsutil cp /tmp/$TIMESTAMP gs://chromium-skia-gm/telemetry/page_sets/slave$SLAVE_NUM/$PAGESETS_TYPE/TIMESTAMP
rm /tmp/$TIMESTAMP

delete_worker_file $CREATING_PAGESETS_ACTIVITY

