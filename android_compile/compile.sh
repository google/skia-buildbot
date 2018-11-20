#! /bin/bash

# Bash script to compile frameworks/base/core/json in an Android checkout.
# This is done via a bash script because ./build/envsetup.sh needs to be
# sourced before running lunch and mmma commands.

set -e

function log_step() {
    echo ""
    echo ""
    echo "============================================"
    echo $1
    echo "============================================"
    echo ""
    echo ""
}


if [[ -z "$1" || -z "$2" || -z "$3" ]]
  then
    echo "Three arguments are required:"
    echo "1. Android checkout directory."
    echo "2. Lunch target."
    echo "3. A comma separated list of targets to build via mmma."
    echo
    echo "Example usage:"
    echo "bash compile.sh /mnt/pd0/checkouts/checkout_1 cf_x86_phone-eng frameworks/base/core/jni,external/skia"
    exit 1
fi
checkout=$1
lunch_target=$2
mmma_targets=$3
cd $checkout

# Set ccache env variables.
export CCACHE_DIR=/mnt/pd0/ccache
export USE_CCACHE=1
export CCACHE_EXEC=/usr/bin/ccache
ccache -M 500G

source_cmd="source ./build/envsetup.sh"
log_step "Running $source_cmd"
eval $source_cmd

lunch_cmd="lunch $lunch_target"
log_step "Running $lunch_cmd"
eval $lunch_cmd

log_step "ccache stats before compilations"
ccache -s

IFS=',' read -ra mmma_targets_arr <<< "$mmma_targets"
for i in "${mmma_targets_arr[@]}"; do
  mmma_cmd="time mmma -j50 $i"
  log_step "Running $mmma_cmd"
  eval $mmma_cmd
done

log_step "ccache stats after compilations"
ccache -s
