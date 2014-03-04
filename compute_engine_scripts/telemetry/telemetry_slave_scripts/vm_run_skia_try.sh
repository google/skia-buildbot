#!/bin/bash
#
# Applies a Skia patch and compares images of SKPs with render_pictures.
#
# The script should be run from the skia-telemetry-slave GCE instance's
# /home/default/skia-repo/buildbot/compute_engine_scripts/telemetry/telemetry_slave_scripts
# directory.
#
# Copyright 2013 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)


function usage() {
  cat << EOF

usage: $0 options

This script runs render pictures on SKPs with the specified patch and then runs
render pictures on SKPs without the patch. The two sets of images are then
compared and a JSON file is outputted detailing all failures.

OPTIONS:
  -h Show this message
  -n The slave_num of this cluster telemetry slave
  -p The location of the Skia patch in Google Storage
  -t The type of pagesets to run against. Eg: All, Filtered, 100k, 10k
  -b Which chromium build the SKPs were created with
  -a Arguments to pass to render_pictures
  -m Whether to build with mesa for the nopatch run
  -w Whether to build with mesa for the withpatch run
  -r The runid (typically requester + timestamp)
  -g The Google Storage location where the log file should be uploaded to
  -o The Google Storage location where the output file should be uploaded to
  -l The location of the log file
EOF
}

while getopts "hn:p:t:b:a:r:m:w:g:o:l:" OPTION
do
  case $OPTION in
    h)
      usage
      exit 1
      ;;
    n)
      SLAVE_NUM=$OPTARG
      ;;
    p)
      SKIA_PATCH_GS_LOCATION=$OPTARG
      ;;
    t)
      PAGESETS_TYPE=$OPTARG
      ;;
    b)
      CHROMIUM_BUILD_DIR=$OPTARG
      ;;
    a)
      RENDER_PICTURES_ARGS=$OPTARG
      ;;
    m)
      MESA_NOPATCH_RUN=$OPTARG
      ;;
    w)
      MESA_WITHPATCH_RUN=$OPTARG
      ;;
    r)
      RUN_ID=$OPTARG
      ;;
    g)
      LOG_FILE_GS_LOCATION=$OPTARG
      ;;
    o)
      OUTPUT_FILE_GS_LOCATION=$OPTARG
      ;;
    l)
      LOG_FILE=$OPTARG
      ;;
    ?)
      usage
      exit
      ;;
  esac
done

if [[ -z $SLAVE_NUM ]] || [[ -z $SKIA_PATCH_GS_LOCATION ]] || \
   [[ -z $PAGESETS_TYPE ]] || [[ -z $CHROMIUM_BUILD_DIR ]] || \
   [[ -z $RENDER_PICTURES_ARGS ]] || [[ -z $MESA_NOPATCH_RUN ]] || \
   [[ -z $MESA_WITHPATCH_RUN ]] || [[ -z $RUN_ID ]] || [[ -z $LOG_FILE ]] || \
   [[ -z $LOG_FILE_GS_LOCATION  ]] || [[ -z $OUTPUT_FILE_GS_LOCATION  ]]
then
  usage
  exit 1
fi

source vm_utils.sh

WORKER_FILE=SKIA-TRY.$RUN_ID
create_worker_file $WORKER_FILE

if [ -e /etc/boto.cfg ]; then                                                   
  # Move boto.cfg since it may interfere with the ~/.boto file.                 
  sudo mv /etc/boto.cfg /etc/boto.cfg.bak                                       
fi

# Download the Skia patch from Google Storage.
SKIA_PATCH_FILE=/tmp/skia-patch.$RUN_ID
gsutil cp $SKIA_PATCH_GS_LOCATION $SKIA_PATCH_FILE

# Download the SKP files from Google Storage if the local TIMESTAMP is out of date.
LOCAL_SKP_DIR=/home/default/storage/skps/$PAGESETS_TYPE/$CHROMIUM_BUILD_DIR
GS_SKP_DIR=gs://chromium-skia-gm/telemetry/skps/slave$SLAVE_NUM/$PAGESETS_TYPE/$CHROMIUM_BUILD_DIR
mkdir -p $LOCAL_SKP_DIR
are_timestamps_equal $LOCAL_SKP_DIR $GS_SKP_DIR
if [ $? -eq 1 ]; then
  gsutil cp $GS_SKP_DIR/* $LOCAL_SKP_DIR
fi

SKIA_TRUNK_LOCATION=/home/default/skia-repo/trunk
TELEMETRY_SLAVE_SCRIPTS_DIR=/home/default/skia-repo/buildbot/compute_engine_scripts/telemetry/telemetry_slave_scripts

function cleanup_slave_before_exit {
  reset_skia_checkout
  copy_log_to_gs
  delete_worker_file $WORKER_FILE
  rm -rf /tmp/*${RUN_ID}* 
  rm -rf /tmp/diffs
  rm -rf /tmp/whitediffs
}

function build_tools {
  if [ "$1" == "True" ]; then
    echo "== Building Skia with mesa =="
    MESA_GYP_DEFINES="skia_mesa=1"
  else
    echo "== Building Skia without mesa =="
    unset MESA_GYP_DEFINES
  fi
  GYP_DEFINES="skia_warnings_as_errors=0 $MESA_GYP_DEFINES" make tools BUILDTYPE=Release
}

function reset_skia_checkout {
    cd $SKIA_TRUNK_LOCATION
    git reset --hard HEAD
    git clean -f -d
}

function run_render_pictures {
  output_dir=$1
  use_mesa=$2
  if [ "$use_mesa" == "True" ]; then
    render_pictures_args_of_run=$(echo $RENDER_PICTURES_ARGS | sed -e 's/--config [a-zA-Z0-9]*/--config mesa/g')
  else
    render_pictures_args_of_run=$RENDER_PICTURES_ARGS
  fi
  ./out/Release/render_pictures -r $LOCAL_SKP_DIR $render_pictures_args_of_run -w $output_dir --writeJsonSummaryPath $output_dir/summary.json
  if [ $? -ne 0 ]; then
    echo "== Failure when running render_pictures. Exiting. =="
    cleanup_slave_before_exit
    exit 1
  fi
}

function copy_log_to_gs {
  gsutil cp -a public-read $LOG_FILE ${LOG_FILE_GS_LOCATION}/slave${SLAVE_NUM}/
}

# Ensure we are starting from a clean checkout and sync.
cd $SKIA_TRUNK_LOCATION
reset_skia_checkout
make clean
for i in {1..3}; do /home/default/depot_tools/gclient sync && break || sleep 2; done

echo "== Applying the patch, building, and running render_pictures =="
PATCH_FILESIZE=$(stat -c%s $SKIA_PATCH_FILE)
if [ $PATCH_FILESIZE != 1 ]; then
  git apply --index -p1 --verbose --ignore-whitespace --ignore-space-change $SKIA_PATCH_FILE
  if [ $? -ne 0 ]; then
      echo "== Patch failed to apply. Exiting. =="
      cleanup_slave_before_exit
      exit 1
  fi
  echo "== Applied patch successfully =="
else
  echo "== Empty patch specified =="
fi
build_tools $MESA_WITHPATCH_RUN
IMG_ROOT=/tmp
OUTPUT_DIR_WITHPATCH=$IMG_ROOT/withpatch-pictures-$RUN_ID
mkdir -p $OUTPUT_DIR_WITHPATCH
run_render_pictures $OUTPUT_DIR_WITHPATCH $MESA_WITHPATCH_RUN

echo "== Removing the patch, building, and running render_pictures =="
reset_skia_checkout
make clean
build_tools $MESA_NOPATCH_RUN
OUTPUT_DIR_NOPATCH=$IMG_ROOT/nopatch-pictures-$RUN_ID
mkdir -p $OUTPUT_DIR_NOPATCH
run_render_pictures $OUTPUT_DIR_NOPATCH $MESA_NOPATCH_RUN

echo "== Comparing pictures and saving differences in JSON output file =="
JSON_SUMMARY_DIR=/tmp/summary-$RUN_ID
mkdir -p $JSON_SUMMARY_DIR
python $TELEMETRY_SLAVE_SCRIPTS_DIR/write_json_summary.py \
  --img_root=$IMG_ROOT \
  --nopatch_json=$OUTPUT_DIR_NOPATCH/summary.json \
  --nopatch_images_base_url=file:/$OUTPUT_DIR_NOPATCH \
  --withpatch_json=$OUTPUT_DIR_WITHPATCH/summary.json \
  --withpatch_images_base_url=file:/$OUTPUT_DIR_WITHPATCH \
  --output_file_path=$JSON_SUMMARY_DIR/slave$SLAVE_NUM.json \
  --gs_output_dir=$OUTPUT_FILE_GS_LOCATION \
  --gs_skp_dir=$GS_SKP_DIR \
  --slave_num=$SLAVE_NUM \
  --gm_json_path=$SKIA_TRUNK_LOCATION/gm/gm_json.py \
  --imagediffdb_path=$SKIA_TRUNK_LOCATION/gm/rebaseline_server/imagediffdb.py

echo "== Copy everything to Google Storage =="
# Get list of failed file names and upload only those to Google Storage.
ARRAY=`cat $JSON_SUMMARY_DIR/slave${SLAVE_NUM}.json | grep 'fileName' | cut -d ':' -f 2 | cut -d "\"" -f2`
for i in ${ARRAY[@]}; do
  gsutil cp $OUTPUT_DIR_NOPATCH/$i $OUTPUT_FILE_GS_LOCATION/slave$SLAVE_NUM/nopatch-images/
  gsutil cp $OUTPUT_DIR_WITHPATCH/$i $OUTPUT_FILE_GS_LOCATION/slave$SLAVE_NUM/withpatch-images/
done
# Copy the diffs and whitediffs to Google Storage.
gsutil cp $IMG_ROOT/diffs/* $OUTPUT_FILE_GS_LOCATION/slave$SLAVE_NUM/diffs/
gsutil cp $IMG_ROOT/whitediffs/* $OUTPUT_FILE_GS_LOCATION/slave$SLAVE_NUM/whitediffs/

# Set google.com permissions on all uploaded images.
gsutil acl ch -g google.com:READ $OUTPUT_FILE_GS_LOCATION/slave$SLAVE_NUM/nopatch-images/*
gsutil acl ch -g google.com:READ $OUTPUT_FILE_GS_LOCATION/slave$SLAVE_NUM/withpatch-images/*
gsutil acl ch -g google.com:READ $OUTPUT_FILE_GS_LOCATION/slave$SLAVE_NUM/diffs/*
gsutil acl ch -g google.com:READ $OUTPUT_FILE_GS_LOCATION/slave$SLAVE_NUM/whitediffs/*
gsutil cp $JSON_SUMMARY_DIR/* $OUTPUT_FILE_GS_LOCATION/slave$SLAVE_NUM/

cleanup_slave_before_exit
