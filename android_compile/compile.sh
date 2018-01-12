#! /bin/bash

# TODO(rmistry):
# Bash script to... finish documentation.

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


if [ -z "$1" ]
  then
    echo "Missing Android checkout directory"
    exit 1
fi
checkout=$1
cd $checkout

source_cmd="source ./build/envsetup.sh"
log_step "Running $source_cmd"
eval $source_cmd

lunch_cmd="lunch gce_x86_phone-eng"
log_step "Running $lunch_cmd"
eval $lunch_cmd

mmma_cmd="mmma -j32 frameworks/base/core/jni"
log_step "Running $mmma_cmd"
eval $mmma_cmd

