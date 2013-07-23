#!/bin/bash
#
# Create pagesets for this slave.
#
# The script should be run from the skia-telemetry-slave GCE instance's
# /home/default/skia-repo/buildbot/compute_engine_scripts/telemetry/telemetry_slave_scripts
# directory.
#
# Copyright 2013 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)

if [ $# -ne 3 ]; then
  echo
  echo "Usage: `basename $0` 1 1 1000"
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
source ../vm_config.sh

create_worker_file $CREATING_PAGESETS_ACTIVITY

# Sync buildbot.
/home/default/depot_tools/gclient sync

# Move into the buildbot/tools directory.
cd ../../../tools
# Delete the old page_sets.
rm -rf page_sets/*.json
# CLean and create directories where page_sets will be stored.
rm -rf ~/storage/page_sets/$PAGESETS_TYPE/*

# If PAGESETS_TYPE is 100k or 10k then adjust NUM_WEBPAGES.
if [ "$PAGESETS_TYPE" == "100k" ]; then
  NUM_WEBPAGES=100000
elif [ "$PAGESETS_TYPE" == "10k" ]; then
  NUM_WEBPAGES=10000
fi

NUM_WEBPAGES_PER_SLAVE=$(($NUM_WEBPAGES/$NUM_SLAVES))
NUM_PAGESETS_PER_SLAVE=$(($NUM_WEBPAGES_PER_SLAVE/$MAX_WEBPAGES_PER_PAGESET))
START=$WEBPAGES_START

# Download the top-1m.csv
wget -O /tmp/pagesets.zip http://s3.amazonaws.com/alexa-static/top-1m.csv.zip
unzip /tmp/pagesets.zip -d page_sets/
rm /tmp/pagesets.zip

# Run create_page_set.py
for PAGESET_NUM in $(seq 1 $NUM_PAGESETS_PER_SLAVE); do
  END=$(expr $START + $MAX_WEBPAGES_PER_PAGESET - 1)
  python create_page_set.py -s $START -e $END -c page_sets/top-1m.csv
  START=$(expr $END + 1)
done
# Copy page_sets to the local directory.
mkdir -p ~/storage/page_sets/$PAGESETS_TYPE
mv page_sets/*.json ~/storage/page_sets/$PAGESETS_TYPE/
# Copy the top-1m.csv to the page_sets directory as well.
mv page_sets/*.csv ~/storage/page_sets/$PAGESETS_TYPE/

if [ -e /etc/boto.cfg ]; then
  # Move boto.cfg since it may interfere with the ~/.boto file.
  sudo mv /etc/boto.cfg /etc/boto.cfg.bak
fi

# Clean the directory in Google Storage.
gsutil rm -R gs://chromium-skia-gm/telemetry/page_sets/slave$SLAVE_NUM/$PAGESETS_TYPE/*
# Copy the page_sets into Google Storage.
gsutil cp ~/storage/page_sets/$PAGESETS_TYPE/* gs://chromium-skia-gm/telemetry/page_sets/slave$SLAVE_NUM/$PAGESETS_TYPE/

# Create a TIMESTAMP file and copy it to Google Storage.
TIMESTAMP=`date +%s`
echo $TIMESTAMP > /tmp/$TIMESTAMP
cp /tmp/$TIMESTAMP ~/storage/page_sets/$PAGESETS_TYPE/TIMESTAMP
gsutil cp /tmp/$TIMESTAMP gs://chromium-skia-gm/telemetry/page_sets/slave$SLAVE_NUM/$PAGESETS_TYPE/TIMESTAMP
rm /tmp/$TIMESTAMP

delete_worker_file $CREATING_PAGESETS_ACTIVITY

