#!/bin/bash
#
# Runs PixelsDiff the specified Chromium/Blink/Skia patches on the GCE slaves.
#
# Copyright 2014 Google Inc. All Rights Reserved.
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
  -e The email address of the requester
  -i The key of the appengine telemetry task
  -l The location of the log file
EOF
}

while getopts "hp:t:s:r:e:i:l:" OPTION
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
   [[ -z $REQUESTER_EMAIL ]] || [[ -z $APPENGINE_KEY ]] || \
   [[ -z $LOG_FILE_LOCATION ]] || [[ -z $SKIA_PATCH_LOCATION ]] || \
   [[ -z $RUN_ID ]]
then
  usage
  exit 1
fi

source ../vm_config.sh
source vm_utils.sh

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

# Create the two required chromium builds (with patch and without the patch).
TIMER="$(date +%s)"
CHROMIUM_BUILD_LOG_FILE=/tmp/try-chromium-build-$RUN_ID
PIXEL_DIFFS=true bash vm_build_chromium_with_patches.sh $CHROMIUM_PATCH_LOCATION \
    $BLINK_PATCH_LOCATION $SKIA_PATCH_LOCATION $RUN_ID \
    $CHROMIUM_BUILD_LOG_FILE 0 &> $CHROMIUM_BUILD_LOG_FILE
ret_value=$?
CHROMIUM_BUILDS_TIME="$(($(date +%s)-TIMER))"

PATCHES_LINK=https://storage.cloud.google.com/chromium-skia-gm/telemetry/tryserver-patches
CHROMIUM_PATCH_LINK=$PATCHES_LINK/$RUN_ID.chromium.patch
BLINK_PATCH_LINK=$PATCHES_LINK/$RUN_ID.blink.patch
SKIA_PATCH_LINK=$PATCHES_LINK/$RUN_ID.skia.patch

# Run pixeldiffs only if the chromium build succeeded.
if [ $ret_value -eq 0 ]; then
  # Find the chromium build directory using the RUN_ID.
  GS_BUILD_PATH=`gsutil ls gs://chromium-skia-gm/telemetry/chromium-builds/try*-$RUN_ID\/* | grep $RUN_ID -m 1`
  CHROMIUM_BUILD_DIR=${GS_BUILD_PATH#*/chromium-builds/}
  # Strip out the filename at the end.
  CHROMIUM_BUILD_DIR=${CHROMIUM_BUILD_DIR%/*}

  # Download the top-1m.csv from Alexa
  echo "=====Downloading list of top 1,000,000 sites from Alexa====="
  wget -O /tmp/top-1m.csv.zip http://s3.amazonaws.com/alexa-static/top-1m.csv.zip
  unzip -o /tmp/top-1m.csv.zip -d /tmp/
  gsutil cp /tmp/top-1m.csv gs://chromium-skia-gm/telemetry/pixeldiffs/csv/top-1m-${RUN_ID}.csv

  # Delete the try chromium builds from the slaves so that they do not take up unneeded disk space.
  bash vm_run_command_on_slaves.sh "rm -rf ~/storage/chromium-builds/try-*"
  # Make sure there are no left over processes on the slaves.
  bash vm_run_command_on_slaves.sh "sudo pkill -9 -f chromium-builds"

  NUM_WEBPAGES_PER_SLAVE=$((10000/$NUM_SLAVES))

  # Start the timer.
  TIMER="$(date +%s)"

  START=1
  for SLAVE_NUM in $(seq 1 $NUM_SLAVES); do
    END=$(expr $START + $NUM_WEBPAGES_PER_SLAVE - 1)
    CMD="bash vm_run_pixeldiffs_try.sh -n $SLAVE_NUM -b $CHROMIUM_BUILD_DIR -p ${CHROMIUM_BUILD_DIR}-withpatch -s $START -e $END -r $RUN_ID -g gs://chromium-skia-gm/telemetry/pixeldiffs/logs/${RUN_ID}/ -o gs://chromium-skia-gm/telemetry/pixeldiffs/outputs/${RUN_ID} -l /tmp/pixeldiffs-${RUN_ID}_output.txt"
    START=$(expr $END + 1)
    ssh -f -X -o UserKnownHostsFile=/dev/null -o CheckHostIP=no \
      -o StrictHostKeyChecking=no -i /home/default/.ssh/google_compute_engine \
      -A -p 22 default@108.170.192.$SLAVE_NUM -- "source .bashrc; cd skia-repo/buildbot/compute_engine_scripts/telemetry/telemetry_slave_scripts; /home/default/depot_tools/gclient sync; $CMD > /tmp/pixeldiffs-${RUN_ID}_output.txt 2>&1"
  done

  # Sleep for a minute to give the slaves some time to start processing.
  sleep 60

  # Check to see if the slaves are done with this pixeldiffs task.
  SLAVES_STILL_PROCESSING=true
  while $SLAVES_STILL_PROCESSING ; do
    SLAVES_STILL_PROCESSING=false
    for SLAVE_NUM in $(seq 1 $NUM_SLAVES); do
      RET=$( is_slave_currently_executing $SLAVE_NUM PIXELDIFFS.${RUN_ID} )
      if $RET; then
        echo "skia-telemetry-worker$SLAVE_NUM is still running PIXELDIFFS.${RUN_ID}"
        echo "Sleeping for a minute and then retrying"
        SLAVES_STILL_PROCESSING=true
        sleep 60
        break
      else
        echo "skia-telemetry-worker$SLAVE_NUM is done processing."
      fi
    done
  done

  # Delete the try chromium builds from the slaves so that they do not take up unneeded disk space.
  bash vm_run_command_on_slaves.sh "rm -rf ~/storage/chromium-builds/try-*"
  # Make sure there are no left over processes on the slaves.
  bash vm_run_command_on_slaves.sh "sudo pkill -9 -f chromium-builds"

  # Create a summary HTML file from the outputs of the slaves.
  HTML_OUTPUT_DIR=/tmp/${RUN_ID}-html/
  mkdir -p $HTML_OUTPUT_DIR
  python ../pixeldiffs_totals_combiner.py \
    --pixeldiffs_gs_root=gs://chromium-skia-gm/telemetry/pixeldiffs/outputs/${RUN_ID}/ \
    --requester_email=$REQUESTER_EMAIL \
    --output_html_dir=$HTML_OUTPUT_DIR \
    --chromium_patch_link=$CHROMIUM_PATCH_LINK \
    --blink_patch_link=$BLINK_PATCH_LINK \
    --skia_patch_link=$SKIA_PATCH_LINK \
    --pixeldiffs_gs_http_path=https://storage.cloud.google.com/chromium-skia-gm/telemetry/pixeldiffs/outputs/${RUN_ID}/

  # Copy the summary HTML to Google Storage.
  gsutil cp -a public-read $HTML_OUTPUT_DIR/index.html gs://chromium-skia-gm/telemetry/pixeldiffs/outputs/${RUN_ID}/

  PIXELDIFFS_TIME="$(($(date +%s)-TIMER))"
else
  PIXELDIFFS_TIME=0
fi

# Email the requester.
CHROMIUM_BUILD_LOG_LINK=https://storage.cloud.google.com/chromium-skia-gm/telemetry/tryserver-outputs/build-logs/$RUN_ID-chromium.txt
HTML_OUTPUT_LINK=https://storage.cloud.google.com/chromium-skia-gm/telemetry/pixeldiffs/outputs/${RUN_ID}/index.html
SLAVE_1_LOG_LINK=https://storage.cloud.google.com/chromium-skia-gm/telemetry/pixeldiffs/logs/${RUN_ID}/slave1/pixeldiffs-${RUN_ID}_output.txt

BOUNDARY=`date +%s|md5sum`
BOUNDARY=${BOUNDARY:0:32}
sendmail $REQUESTER_EMAIL,$ADMIN_EMAIL <<EOF
subject:Your PixelDiff Cluster Telemetry trybot run has completed! ($RUN_ID)
to:$REQUESTER_EMAIL,$ADMIN_EMAIL
from:skia.buildbot@gmail.com
Content-Type: multipart/mixed; boundary=\"$BOUNDARY\";

This is a MIME-encapsulated message

--$BOUNDARY
Content-Type: text/html

<html>
  <head/>
  <body>

  The HTML output with pixel differences between the base run and the patch run is <a href='$HTML_OUTPUT_LINK'>here</a>.<br/>
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
      <td>Pixel diffs run</td>
      <td>$PIXELDIFFS_TIME secs</td>
      <td><a href='$SLAVE_1_LOG_LINK'>log</a></td>
    </tr>
  </table><br/><br/>

  You can schedule more runs <a href='https://skia-tree-status.appspot.com/skia-telemetry/chromium_try'>here</a>.<br/><br/>
  Thanks!
  </body>
</html>

--$BOUNDARY--

EOF

# Mark this task as completed on AppEngine.
PASSWORD=`cat /home/default/skia-repo/buildbot/compute_engine_scripts/telemetry/telemetry_master_scripts/appengine_password.txt`
for i in {1..10}; do wget --post-data "key=$APPENGINE_KEY&chromium_patch_link=$CHROMIUM_PATCH_LINK&blink_patch_link=$BLINK_PATCH_LINK&skia_patch_link=$SKIA_PATCH_LINK&build_log_link=$CHROMIUM_BUILD_LOG_LINK&telemetry_nopatch_log_link=$SLAVE_1_LOG_LINK&telemetry_withpatch_log_link=$SLAVE_1_LOG_LINK&html_output_link=$HTML_OUTPUT_LINK&password=$PASSWORD" "https://skia-tree-status.appspot.com/skia-telemetry/update_chromium_try_tasks" -O /dev/null && break || sleep 2; done

# Copy log file to Google Storage.
gsutil cp -a public-read $LOG_FILE_LOCATION gs://chromium-skia-gm/telemetry/pixeldiffs/logs/${RUN_ID}/

# Delete output files.
rm -rf /tmp/*${RUN_ID}*
rm -rf /tmp/top-1m.csv*

