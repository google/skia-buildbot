#!/bin/bash
#
# Runs the specified chromium patch on the GCE slaves.
#
# Copyright 2013 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)


function usage() {
  cat << EOF

usage: $0 options

This script runs the specified chromium patch on the GCE slaves.

OPTIONS:
  -h Show this message
  -p The location of the Chromium patch
  -t The location of the Blink patch
  -s The location of the Skia patch
  -r The unique runid (typically requester + timestamp)
  -v The variance threshold for comparing the resultant CSV files
  -o The percentage of outliers to discard when comparing the result CSV files
  -b The telemetry benchmark to run on this slave
  -a The extra arguments that the telemetry benchmark needs
  -e The email address of the requester
  -i The key of the appengine telemetry task
  -l The location of the log file
EOF
}

while getopts "hp:t:s:r:v:o:b:a:e:i:l:" OPTION
do
  case $OPTION in
    h)
      usage
      exit 1
      ;;
    p)
      CHROMIUM_PATCH_LOCATION=$OPTARG
      ;;
    t)
      BLINK_PATCH_LOCATION=$OPTARG
      ;;
    s)
      SKIA_PATCH_LOCATION=$OPTARG
      ;;
    r)
      RUN_ID=$OPTARG
      ;;
    v)
      VARIANCE_THRESHOLD=$OPTARG
      ;;
    o)
      DISCARD_OUTLIERS=$OPTARG
      ;;
    b)
      TELEMETRY_BENCHMARK=$OPTARG
      ;;
    a)
      EXTRA_ARGS=$OPTARG
      ;;
    e)
      REQUESTER_EMAIL=$OPTARG
      ;;
    i)
      APPENGINE_KEY=$OPTARG
      ;;
    l)
      LOG_FILE_LOCATION=$OPTARG
      ;;
    ?)
      usage
      exit
      ;;
  esac
done

if [[ -z $CHROMIUM_PATCH_LOCATION ]] || [[ -z $BLINK_PATCH_LOCATION ]] || \
   [[ -z $VARIANCE_THRESHOLD ]] || [[ -z $DISCARD_OUTLIERS ]] || \
   [[ -z $TELEMETRY_BENCHMARK ]] || [[ -z $EXTRA_ARGS ]] || \
   [[ -z $REQUESTER_EMAIL ]] || [[ -z $APPENGINE_KEY ]] || \
   [[ -z $LOG_FILE_LOCATION ]] || [[ -z $SKIA_PATCH_LOCATION ]]
then
  usage
  exit 1
fi

source ../vm_config.sh

# Update buildbot.
for i in {1..3}; do gclient sync && break || sleep 2; done

# Copy the patch to Google Storage.
PATCHES_GS_LOCATION=gs://chromium-skia-gm/telemetry/tryserver-patches
CHROMIUM_PATCH_GS_LOCATION=$PATCHES_GS_LOCATION/$RUN_ID.chromium.patch
BLINK_PATCH_GS_LOCATION=$PATCHES_GS_LOCATION/$RUN_ID.blink.patch
SKIA_PATCH_GS_LOCATION=$PATCHES_GS_LOCATION/$RUN_ID.skia.patch
gsutil cp -a public-read $CHROMIUM_PATCH_LOCATION $CHROMIUM_PATCH_GS_LOCATION
gsutil cp -a public-read $BLINK_PATCH_LOCATION $BLINK_PATCH_GS_LOCATION
gsutil cp -a public-read $SKIA_PATCH_LOCATION $SKIA_PATCH_GS_LOCATION

# If it is a rasterize_and_record_micro benchmark request then use aura.
if [ "$TELEMETRY_BENCHMARK" == "rasterize_and_record_micro" ]; then
  USE_AURA=1
  EXTRA_ARGS="--rasterize-repeat=200 --record-repeat=200 $EXTRA_ARGS"
else
  USE_AURA=0
fi

# Create the two required chromium builds (with patch and without the patch).
TIMER="$(date +%s)"
CHROMIUM_BUILD_LOG_FILE=/tmp/try-chromium-build-$RUN_ID
bash vm_build_chromium_with_patches.sh $CHROMIUM_PATCH_LOCATION \
    $BLINK_PATCH_LOCATION $SKIA_PATCH_LOCATION $RUN_ID \
    $CHROMIUM_BUILD_LOG_FILE $USE_AURA &> $CHROMIUM_BUILD_LOG_FILE
ret_value=$?
CHROMIUM_BUILDS_TIME="$(($(date +%s)-TIMER))"

# Run telemetry benchmarks only if the chromium build succeeded.
if [ $ret_value -eq 0 ]; then
  # Find the chromium build directory using the RUN_ID.
  GS_BUILD_PATH=`gsutil ls gs://chromium-skia-gm/telemetry/chromium-builds/try*-$RUN_ID\/* | grep $RUN_ID -m 1`
  CHROMIUM_BUILD_DIR=${GS_BUILD_PATH#*/chromium-builds/}
  # Strip out the filename at the end.
  CHROMIUM_BUILD_DIR=${CHROMIUM_BUILD_DIR%/*}

  # Reboot all slaves so that they start from a clean slate.
  bash vm_run_command_on_slaves.sh "sudo reboot"
  # Sleep for 4 mins for all slaves to come back up.
  sleep 240

  # Run telemetry on the slaves using the specified benchmark.
  TELEMETRY_BUILD_LOG=/tmp/try-telemetry-nopatch-$RUN_ID
  TELEMETRY_NOPATCH_ID=$RUN_ID-nopatch
  TIMER="$(date +%s)"
  TRYSERVER=true bash vm_run_telemetry_on_slaves.sh $TELEMETRY_BENCHMARK \
      "$EXTRA_ARGS" 10k $CHROMIUM_BUILD_DIR $TELEMETRY_NOPATCH_ID \
      $REQUESTER_EMAIL $APPENGINE_KEY $TELEMETRY_BUILD_LOG &> $TELEMETRY_BUILD_LOG
  TELEMETRY_WITHOUT_PATCH_TIME="$(($(date +%s)-TIMER))"

  # Reboot all slaves so that they start from a clean slate.
  bash vm_run_command_on_slaves.sh "sudo reboot"
  # Sleep for 4 mins for all slaves to come back up.
  sleep 240

  # Run telemetry using the patch build.
  TELEMETRY_BUILD_LOG=/tmp/try-telemetry-withpatch-$RUN_ID
  TELEMETRY_WITHPATCH_ID=$RUN_ID-withpatch
  TIMER="$(date +%s)"
  TRYSERVER=true bash vm_run_telemetry_on_slaves.sh $TELEMETRY_BENCHMARK \
      "$EXTRA_ARGS" 10k $CHROMIUM_BUILD_DIR-withpatch $TELEMETRY_WITHPATCH_ID \
      $REQUESTER_EMAIL $APPENGINE_KEY $TELEMETRY_BUILD_LOG &> $TELEMETRY_BUILD_LOG
  TELEMETRY_WITH_PATCH_TIME="$(($(date +%s)-TIMER))"

  # Delete the try chromium builds from the slaves so that they do not take up unneeded disk space.
  bash vm_run_command_on_slaves.sh "rm -rf ~/storage/chromium-builds/try-*"
  # Make sure there are no left over processes on the slaves.
  bash vm_run_command_on_slaves.sh "sudo pkill -9 -f chromium-builds"


  # Compare the resultant CSV files.
  NOPATCH_CSV="/home/default/storage/telemetry_outputs/${TELEMETRY_NOPATCH_ID}/${TELEMETRY_NOPATCH_ID}.$TELEMETRY_BENCHMARK.output"
  WITHPATCH_CSV="/home/default/storage/telemetry_outputs/${TELEMETRY_WITHPATCH_ID}/${TELEMETRY_WITHPATCH_ID}.$TELEMETRY_BENCHMARK.output"
  HTML_OUTPUT_DIR="/tmp/html-$RUN_ID"
  HTML_OUTPUT_LINK_BASE=https://storage.cloud.google.com/chromium-skia-gm/telemetry/tryserver-outputs/html-outputs/$RUN_ID/
  mkdir -p $HTML_OUTPUT_DIR
  cd ..
  python csv_comparer.py --csv_file1=$NOPATCH_CSV --csv_file2=$WITHPATCH_CSV --output_html=$HTML_OUTPUT_DIR --variance_threshold=$VARIANCE_THRESHOLD --discard_outliers=$DISCARD_OUTLIERS --absolute_url=$HTML_OUTPUT_LINK_BASE

  # Copy the HTML files to Google Storage.
  gsutil cp -a public-read $HTML_OUTPUT_DIR/*.html gs://chromium-skia-gm/telemetry/tryserver-outputs/html-outputs/$RUN_ID/
else
  TELEMETRY_WITHOUT_PATCH_TIME=0
  TELEMETRY_WITH_PATCH_TIME=0
  # Create a dummy link when there are failures.
  HTML_OUTPUT_LINK_BASE='http://google.com/'
fi

# Email the requester.
HTML_OUTPUT_LINK=${HTML_OUTPUT_LINK_BASE}index.html
CHROMIUM_BUILD_LOG_LINK=https://storage.cloud.google.com/chromium-skia-gm/telemetry/tryserver-outputs/build-logs/$RUN_ID-chromium.txt
PATCHES_LINK=https://storage.cloud.google.com/chromium-skia-gm/telemetry/tryserver-patches
CHROMIUM_PATCH_LINK=$PATCHES_LINK/$RUN_ID.chromium.patch
BLINK_PATCH_LINK=$PATCHES_LINK/$RUN_ID.blink.patch
SKIA_PATCH_LINK=$PATCHES_LINK/$RUN_ID.skia.patch
TELEMETRY_OUTPUT_1=https://storage.cloud.google.com/chromium-skia-gm/telemetry/benchmarks/$TELEMETRY_BENCHMARK/consolidated-outputs/$TELEMETRY_NOPATCH_ID.output.txt
TELEMETRY_OUTPUT_2=https://storage.cloud.google.com/chromium-skia-gm/telemetry/benchmarks/$TELEMETRY_BENCHMARK/consolidated-outputs/$TELEMETRY_WITHPATCH_ID.output.txt
SLAVE_1_LOG_LINK=https://storage.cloud.google.com/chromium-skia-gm/telemetry/benchmarks/$TELEMETRY_BENCHMARK/slave1/logs/$RUN_ID-withpatch.log

BOUNDARY=`date +%s|md5sum`
BOUNDARY=${BOUNDARY:0:32}
sendmail $REQUESTER_EMAIL,$ADMIN_EMAIL <<EOF
subject:Your Cluster Telemetry trybot run has completed! ($RUN_ID)
to:$REQUESTER_EMAIL,$ADMIN_EMAIL
from:skia.buildbot@gmail.com
Content-Type: multipart/mixed; boundary=\"$BOUNDARY\";

This is a MIME-encapsulated message

--$BOUNDARY
Content-Type: text/html

<html>
  <head/>
  <body>

  The HTML output with differences between the base run and the patch run is <a href='$HTML_OUTPUT_LINK'>here</a>.<br/>
  The patch(es) you specified are here: 
  <a href='$CHROMIUM_PATCH_LINK'>chromium</a>/<a href='$BLINK_PATCH_LINK'>blink</a>/<a href='$SKIA_PATCH_LINK'>skia</a>
  <br/><br/>

  <table border="1" cellpadding="5">
    <tr>
      <th>Task</th>
      <th>Time Taken</th>
      <th>Log</th>
    </tr>
    <tr>
      <td>Chromium Builds (with and without patch)</td>
      <td>$CHROMIUM_BUILDS_TIME secs</td>
      <td><a href='$CHROMIUM_BUILD_LOG_LINK'>log</a></td>
    </tr>
    <tr>
      <td>Telemetry run without patch</td>
      <td>$TELEMETRY_WITHOUT_PATCH_TIME secs</td>
      <td><a href='$TELEMETRY_OUTPUT_1'>log</a></td>
    </tr>
    <tr>
      <td>Telemetry run with patch</td>
      <td>$TELEMETRY_WITH_PATCH_TIME secs</td>
      <td><a href='$TELEMETRY_OUTPUT_2'>log</a></td>
    </tr>
  </table><br/><br/>

  The log file of the first slave is <a href='$SLAVE_1_LOG_LINK'>here</a>.<br/>
  You can schedule more runs <a href='https://skia-tree-status.appspot.com/skia-telemetry/chromium_try'>here</a>.<br/><br/>
  Thanks!
  </body>
</html>

--$BOUNDARY--

EOF

# Mark this task as completed on AppEngine.
PASSWORD=`cat /home/default/skia-repo/buildbot/compute_engine_scripts/telemetry/telemetry_master_scripts/appengine_password.txt`
for i in {1..10}; do wget --post-data "key=$APPENGINE_KEY&chromium_patch_link=$CHROMIUM_PATCH_LINK&blink_patch_link=$BLINK_PATCH_LINK&skia_patch_link=$SKIA_PATCH_LINK&build_log_link=$CHROMIUM_BUILD_LOG_LINK&telemetry_nopatch_log_link=$TELEMETRY_OUTPUT_1&telemetry_withpatch_log_link=$TELEMETRY_OUTPUT_2&html_output_link=$HTML_OUTPUT_LINK&password=$PASSWORD" "https://skia-tree-status.appspot.com/skia-telemetry/update_chromium_try_tasks" -O /dev/null && break || sleep 2; done

# Copy log file to Google Storage.
gsutil cp -a public-read $LOG_FILE_LOCATION gs://chromium-skia-gm/telemetry/tryserver-logs/

# Delete all tmp files.
rm -rf /tmp/*${RUN_ID}*
rm -rf /home/default/storage/telemetry_outputs/${RUN_ID}*

