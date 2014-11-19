#!/bin/bash
#
# Runs all steps in vm_setup_slave.sh, calls run_multipage_benchmarks and copies
# the output (if any) to Google Storage.
#
# The script should be run from the cluster-telemetry-slave GCE instance's
# /b/skia-repo/buildbot/cluster_telemetry/telemetry_slave_scripts
# directory.
#
# Copyright 2013 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)


if [ $# -lt 5 ]; then
  echo
  echo "Usage: `basename $0` 1 skpicture_printer" \
    "--skp-outdir=/b/storage/skps/ All a1234b-c5678d" \
    "rmistry2013-05-24.07-34-05"
  echo
  echo "The first argument is the slave_num of this telemetry slave."
  echo "The second argument is the telemetry benchmark to run on this slave."
  echo "The third argument are the extra arguments that the benchmark needs."
  echo "The fourth argument is the type of pagesets to create from the 1M list" \
       "Eg: All, Filtered, 100k, 10k, Deeplinks."
  echo "The fifth argument is the number of times the benchmark should be repeated."
  echo "The sixth argument is the platform the benchmark should run on (Android / Linux)."
  echo "The seventh argument is the name of the directory where the chromium" \
       "build which will be used for this run is stored."
  echo "The eighth argument is the runid (typically requester + timestamp)."
  echo "The ninth argument is the extra browser arguments to send to telemetry."
  echo "The tenth optional argument is the Google Storage location of the URL" \
       "whitelist."
  echo
  exit 1
fi

SLAVE_NUM=$1
TELEMETRY_BENCHMARK=$2
EXTRA_ARGS=$3
PAGESETS_TYPE=$4
REPEAT_TELEMETRY_RUNS=$5
TARGET_PLATFORM=$6
CHROMIUM_BUILD_DIR=$7
RUN_ID=$8
EXTRA_BROWSER_ARGS=$9
WHITELIST_GS_LOCATION=${10}

WHITELIST_FILE=whitelist.$RUN_ID

if [ "$TARGET_PLATFORM" == "Android" ]; then
  # Verify that this slave has adb installed, if it does not then exit.
  adb version
  if [ $? -ne 0 ]; then
    echo "adb not installed. Exiting.."
    gsutil cp -a public-read /tmp/${TELEMETRY_BENCHMARK}-${RUN_ID}_output.txt gs://chromium-skia-gm/telemetry/benchmarks/$TELEMETRY_BENCHMARK/slave$SLAVE_NUM/logs/${RUN_ID}.log
    exit 1
  fi
  OFFLINE_DEVICES=`adb devices | grep offline`
  MISSING_DEVICES=`adb devices | grep device | grep -v devices`
  if [ "$OFFLINE_DEVICES" != "" ]; then
    echo "Android device is offline. Exiting.."
    gsutil cp -a public-read /tmp/${TELEMETRY_BENCHMARK}-${RUN_ID}_output.txt gs://chromium-skia-gm/telemetry/benchmarks/$TELEMETRY_BENCHMARK/slave$SLAVE_NUM/logs/${RUN_ID}.log
    exit 1
  elif [ "$MISSING_DEVICES" == "" ]; then
    echo "Android device is missing. Exiting.."
    gsutil cp -a public-read /tmp/${TELEMETRY_BENCHMARK}-${RUN_ID}_output.txt gs://chromium-skia-gm/telemetry/benchmarks/$TELEMETRY_BENCHMARK/slave$SLAVE_NUM/logs/${RUN_ID}.log
    exit 1
  fi
  # Make sure adb shell is running as root.
  adb root
fi


source vm_utils.sh

create_worker_file TELEMETRY_${RUN_ID}

source vm_setup_slave.sh

# Install the APK on the Android device.
adb install -r /b/storage/chromium-builds/${CHROMIUM_BUILD_DIR}/apks/ChromeShell.apk

# Download the webpage_archives from Google Storage if the local TIMESTAMP is
# out of date.
mkdir -p /b/storage/webpages_archive/$PAGESETS_TYPE/
are_timestamps_equal /b/storage/webpages_archive/$PAGESETS_TYPE gs://chromium-skia-gm/telemetry/webpages_archive/slave$SLAVE_NUM/$PAGESETS_TYPE
if [ $? -eq 1 ]; then
  gsutil -m cp gs://chromium-skia-gm/telemetry/webpages_archive/slave$SLAVE_NUM/$PAGESETS_TYPE/* \
    /b/storage/webpages_archive/$PAGESETS_TYPE
fi

if [[ ! -z "$WHITELIST_GS_LOCATION" ]]; then
  # Copy the whitelist from Google Storage to /tmp.
  gsutil cp $WHITELIST_GS_LOCATION /tmp/$WHITELIST_FILE
fi

# The number of times to repeat telemetry page_set runs.

if [ "$TELEMETRY_BENCHMARK" == "skpicture_printer" ]; then
  # Clean and create the skp output directory.
  sudo chown -R chrome-bot:chrome-bot /b/storage/skps/$PAGESETS_TYPE/$CHROMIUM_BUILD_DIR
  rm -rf /b/storage/skps/$PAGESETS_TYPE/$CHROMIUM_BUILD_DIR
  mkdir -p /b/storage/skps/$PAGESETS_TYPE/$CHROMIUM_BUILD_DIR/
  EXTRA_ARGS="--skp-outdir=/b/storage/skps/$PAGESETS_TYPE/$CHROMIUM_BUILD_DIR/ $EXTRA_ARGS"
  # Only do one run for SKPs.
  REPEAT_TELEMETRY_RUNS=1
fi

if [ "$TELEMETRY_BENCHMARK" == "smoothness" ]; then
  # A synthetic scroll needs to be able to output at least two frames. Make the
  # viewport size smaller than the page size.
  EXTRA_BROWSER_ARGS="--window-size=1280,512 $EXTRA_BROWSER_ARGS"
fi

# Increase the timeout only when pagesets are used which have multiple pages in 
# them and could thus take longer than the default timeout.
if [ "$PAGESETS_TYPE" == "KeyMobileSites" -o "$PAGESETS_TYPE" == "KeySilkCases" ]; then
  TELEMETRY_TIMEOUT=2700
elif [ "$PAGESETS_TYPE" == "GPURasterSet" ]; then
  TELEMETRY_TIMEOUT=1800
else
  # Was 300.
  TELEMETRY_TIMEOUT=1200
fi

OUTPUT_DIR=/b/storage/telemetry_outputs/$RUN_ID
mkdir -p $OUTPUT_DIR

# Start the timer.
TIMER="$(date +%s)"

sudo chown -R chrome-bot:chrome-bot src/webpagereplay_logs

for page_set in /b/storage/page_sets/$PAGESETS_TYPE/*.py; do
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
    # Copy the page set into the page_sets directory for run_measurement to find.
    page_set_filename="${page_set_basename%.*}"
    cp $page_set src/tools/perf/page_sets/$page_set_basename

    for current_run in `seq 1 $REPEAT_TELEMETRY_RUNS`;
    do
      if [ "$TELEMETRY_BENCHMARK" != "skpicture_printer" ]; then
         # Need to capture output for all benchmarks except skpicture_printer.
         OUTPUT_DIR_ARG="-o $OUTPUT_DIR/${RUN_ID}.${page_set_basename}.${current_run}"
      fi
      echo "This is run number $current_run"
      if [ "$TARGET_PLATFORM" == "Android" ]; then
        # Removing throttling to see if we get consistent results.
        # Enable CPU and GPU throttling.
        # adb shell "stop mpdecision; echo 1 > /sys/devices/system/cpu/cpu1/online; echo 1 > /sys/devices/system/cpu/cpu2/online; echo 1 > /sys/devices/system/cpu/cpu3/online; echo 1190400 > /sys/devices/system/cpu/cpu0/cpufreq/scaling_max_freq; echo 1190400 > /sys/devices/system/cpu/cpu1/cpufreq/scaling_max_freq; echo 1190400 > /sys/devices/system/cpu/cpu2/cpufreq/scaling_max_freq; echo 1190400 > /sys/devices/system/cpu/cpu3/cpufreq/scaling_max_freq; echo 200000000 > /sys/class/kgsl/kgsl-3d0/max_gpuclk"
        echo "== Running eval DISPLAY=:0 timeout $TELEMETRY_TIMEOUT src/tools/perf/run_measurement --extra-browser-args=\"$EXTRA_BROWSER_ARGS\" --browser=android-chrome-shell $TELEMETRY_BENCHMARK $page_set_filename $EXTRA_ARGS $OUTPUT_DIR_ARG =="
        eval DISPLAY=:0 timeout $TELEMETRY_TIMEOUT src/tools/perf/run_measurement --extra-browser-args=\"$EXTRA_BROWSER_ARGS\" --browser=android-chrome-shell $TELEMETRY_BENCHMARK $page_set_filename $EXTRA_ARGS $OUTPUT_DIR_ARG
      else
        echo "== Running eval DISPLAY=:0 timeout $TELEMETRY_TIMEOUT src/tools/perf/run_measurement --extra-browser-args=\"$EXTRA_BROWSER_ARGS\" --browser-executable=/b/storage/chromium-builds/${CHROMIUM_BUILD_DIR}/chrome --browser=exact $TELEMETRY_BENCHMARK $page_set_filename $EXTRA_ARGS $OUTPUT_DIR_ARG =="
        eval DISPLAY=:0 timeout $TELEMETRY_TIMEOUT src/tools/perf/run_measurement --extra-browser-args=\"$EXTRA_BROWSER_ARGS\" --browser-executable=/b/storage/chromium-builds/${CHROMIUM_BUILD_DIR}/chrome --browser=exact $TELEMETRY_BENCHMARK $page_set_filename $EXTRA_ARGS $OUTPUT_DIR_ARG
      fi
      sudo chown chrome-bot:chrome-bot $OUTPUT_DIR/${RUN_ID}.${page_set_basename}.${current_run}
    done

    # Delete the page set from the page sets directory now that we are done with
    # it.
    if [ "$PAGESETS_TYPE" == "KeyMobileSites" -o "$PAGESETS_TYPE" == "KeySilkCases" ]; then
      # Do not delete mobile or silk page sets.
      echo
    else
      rm src/tools/perf/page_sets/$page_set_basename
    fi
    if [ $? -eq 124 ]; then
      echo "========== $page_set timed out! =========="
    else
      echo "========== Done with $page_set =========="
    fi
  fi
done

TELEMETRY_TIME="$(($(date +%s)-TIMER))"
echo "Going through all page_sets took $TELEMETRY_TIME seconds"

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
  python /b/skia-repo/buildbot/cluster_telemetry/csv_merger.py \
    --csv_dir=$OUTPUT_DIR/${RUN_ID} --output_csv_name=$OUTPUT_DIR/output.${RUN_ID}
fi

# Copy the consolidated output to Google Storage.
gsutil cp $OUTPUT_DIR/output.${RUN_ID} gs://chromium-skia-gm/telemetry/benchmarks/$TELEMETRY_BENCHMARK/slave$SLAVE_NUM/outputs/${RUN_ID}.output
# Copy the complete telemetry log to Google Storage.
gsutil cp -a public-read /tmp/${TELEMETRY_BENCHMARK}-${RUN_ID}_output.txt gs://chromium-skia-gm/telemetry/benchmarks/$TELEMETRY_BENCHMARK/slave$SLAVE_NUM/logs/${RUN_ID}.log

# Special handling for skpicture_printer, SKP files need to be copied to Google Storage.
if [ "$TELEMETRY_BENCHMARK" == "skpicture_printer" ]; then
  gsutil rm -R gs://chromium-skia-gm/telemetry/skps/slave$SLAVE_NUM/$PAGESETS_TYPE/$CHROMIUM_BUILD_DIR/*
  sudo chown -R chrome-bot:chrome-bot /b/storage/skps/$PAGESETS_TYPE/$CHROMIUM_BUILD_DIR
  cd /b/storage/skps/$PAGESETS_TYPE/$CHROMIUM_BUILD_DIR
  SKP_LIST=`find . -mindepth 1 -maxdepth 1 -type d  \( ! -iname ".*" \) | sed 's|^\./||g'`
  for SKP in $SKP_LIST; do
    echo "Contents of SKP directory are:"
    ls -l /b/storage/skps/$PAGESETS_TYPE/$CHROMIUM_BUILD_DIR/$SKP/
    echo "The largest file here is:"
    SKP_OUTPUT=`find /b/storage/skps/$PAGESETS_TYPE/$CHROMIUM_BUILD_DIR/$SKP/* -printf '%s %p\n' | sort -nr | head -1`
    echo $SKP_OUTPUT
    IFS=' ' read -r id LARGEST_SKP <<< "$SKP_OUTPUT"
    echo $LARGEST_SKP
    # We are only interested in the largest SKP, move it into the SKP repository.
    mv $LARGEST_SKP /b/storage/skps/$PAGESETS_TYPE/$CHROMIUM_BUILD_DIR/$SKP.skp
  done

  # Leave only SKP files in the skps directory.
  rm -rf /b/storage/skps/$PAGESETS_TYPE/$CHROMIUM_BUILD_DIR/*/

  # Delete all SKP files less than 10K (they will be the ones with errors).
  find . -type f -size -10k
  find . -type f -size -10k -exec rm {} \;

  # Remove invalid SKPs found using the skpinfo binary.
  # Sync trunk and build tools.
  cd /b/skia-repo/trunk
  for i in {1..3}; do /b/depot_tools/gclient sync && break || sleep 2; done
  make clean
  GYP_DEFINES="skia_warnings_as_errors=0" make tools BUILDTYPE=Release
  echo "=====Calling remove_invalid_skps.py====="
  cd /b/skia-repo/buildbot/cluster_telemetry/telemetry_slave_scripts
  python remove_invalid_skps.py --skp_dir=/b/storage/skps/$PAGESETS_TYPE/$CHROMIUM_BUILD_DIR/ --path_to_skpinfo=/b/skia-repo/trunk/out/Release/skpinfo

  # Now copy the SKP files to Google Storage.
  gsutil -m cp /b/storage/skps/$PAGESETS_TYPE/$CHROMIUM_BUILD_DIR/* \
    gs://chromium-skia-gm/telemetry/skps/slave$SLAVE_NUM/$PAGESETS_TYPE/$CHROMIUM_BUILD_DIR/
  # Set ACLs for only google.com accounts to read the SKPs.
  gsutil -m acl ch -g google.com:READ gs://chromium-skia-gm/telemetry/skps/slave$SLAVE_NUM/$PAGESETS_TYPE/$CHROMIUM_BUILD_DIR/*

  # Create a TIMESTAMP file and copy it to Google Storage.
  TIMESTAMP=`date +%s`
  echo $TIMESTAMP > /tmp/$TIMESTAMP
  cp /tmp/$TIMESTAMP /b/storage/skps/$PAGESETS_TYPE/$CHROMIUM_BUILD_DIR/TIMESTAMP
  gsutil cp /tmp/$TIMESTAMP gs://chromium-skia-gm/telemetry/skps/slave$SLAVE_NUM/$PAGESETS_TYPE/$CHROMIUM_BUILD_DIR/TIMESTAMP
  rm /tmp/$TIMESTAMP
fi

# Clean up logs and the worker file.
rm -rf ${OUTPUT_DIR}*
delete_worker_file TELEMETRY_${RUN_ID}
