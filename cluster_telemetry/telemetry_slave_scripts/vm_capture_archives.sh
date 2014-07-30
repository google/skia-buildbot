#!/bin/bash
#
# Runs all steps in vm_setup_slave.sh, executes record_wpr and copies the
# created archives to Google Storage.
#
# The script should be run from the cluster-telemetry-slave GCE instance's
# /b/skia-repo/buildbot/cluster_telemetry/telemetry_slave_scripts
# directory.
#
# Copyright 2013 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)


if [ $# -ne 3 ]; then
  echo
  echo "Usage: `basename $0` 1 All a1234b-c5678d"
  echo
  echo "The first argument is the slave_num of this telemetry slave."
  echo "The second argument is the type of pagesets to create from the 1M list"\
       "Eg: All, Filtered, 100k, 10k, Deeplinks."
  echo "The third argument is the name of the directory where the chromium" \
       "build which will be used for this run is stored."
  echo
  exit 1
fi

SLAVE_NUM=$1
PAGESETS_TYPE=$2
CHROMIUM_BUILD_DIR=$3

source ../config.sh
source vm_utils.sh

create_worker_file $RECORD_WPR_ACTIVITY

source vm_setup_slave.sh

# Create the webpages_archive directory.
mkdir -p /b/storage/webpages_archive/$PAGESETS_TYPE/
rm -rf /b/storage/webpages_archive/$PAGESETS_TYPE/*

# Increase the timeout only when pagesets are used which have multiple pages in 
# them and could thus take longer than the default timeout. Also, specify the
# appropriate names for the page sets when calling record_wpr.
if [ "$PAGESETS_TYPE" == "KeyMobileSites" ]; then
  RECORD_TIMEOUT=2700                                                        
  PAGESET_NAME="key_mobile_sites_page_set"                                                         
elif [ "$PAGESETS_TYPE" == "KeySilkCases" ]; then
  RECORD_TIMEOUT=2700                                                        
  PAGESET_NAME="key_silk_cases_page_set"                                                         
elif [ "$PAGESETS_TYPE" == "GPURasterSet" ]; then                               
  RECORD_TIMEOUT=1800                                                        
  PAGESET_NAME="gpu_rasterization_page_set"                                                         
else                                                                            
  RECORD_TIMEOUT=300
  PAGESET_NAME="typical_alexa_page_set"                                                         
fi

for page_set in /b/storage/page_sets/$PAGESETS_TYPE/*.py; do
  if [[ -f $page_set ]]; then
    echo "========== Processing $page_set =========="
    pageset_basename=`basename $page_set`
    if [ "$PAGESETS_TYPE" == "Filtered" ]; then
      # Since the archive already exists in 'All' do not run record_wpr.
      pageset_filename="${pageset_basename%.*}"
      cp  /b/storage/webpages_archive/All/${pageset_filename}* /b/storage/webpages_archive/$PAGESETS_TYPE/
      echo "========== $page_set copied over from All =========="
    else
      # Copy the page set into the page_sets directory for record_wpr to find.
      cp $page_set src/tools/perf/page_sets/$pageset_basename
      sudo DISPLAY=:0 timeout $RECORD_TIMEOUT src/tools/perf/record_wpr --extra-browser-args=--disable-setuid-sandbox --browser-executable=/b/storage/chromium-builds/${CHROMIUM_BUILD_DIR}/chrome --browser=exact $PAGESET_NAME
      # Delete the page set from the page_sets directory now that we are done
      # with it.
      rm  src/tools/perf/page_sets/$pageset_basename
      if [ $? -eq 124 ]; then
        echo "========== $page_set timed out! =========="
      else
        echo "========== Done with $page_set =========="
      fi
    fi
  fi
done

# Copy the webpages_archive directory to Google Storage.
gsutil rm -R gs://chromium-skia-gm/telemetry/webpages_archive/slave$SLAVE_NUM/$PAGESETS_TYPE/*
sudo chown -R chrome-bot:chrome-bot /b/storage/webpages_archive/$PAGESETS_TYPE
gsutil -m cp /b/storage/webpages_archive/$PAGESETS_TYPE/* \
  gs://chromium-skia-gm/telemetry/webpages_archive/slave$SLAVE_NUM/$PAGESETS_TYPE/

# Create a TIMESTAMP file and copy it to Google Storage.
TIMESTAMP=`date +%s`
echo $TIMESTAMP > /tmp/$TIMESTAMP
cp /tmp/$TIMESTAMP /b/storage/webpages_archive/$PAGESETS_TYPE/TIMESTAMP
gsutil cp /tmp/$TIMESTAMP gs://chromium-skia-gm/telemetry/webpages_archive/slave$SLAVE_NUM/$PAGESETS_TYPE/TIMESTAMP
rm /tmp/$TIMESTAMP

delete_worker_file $RECORD_WPR_ACTIVITY
