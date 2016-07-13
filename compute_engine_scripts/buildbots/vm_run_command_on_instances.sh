#!/bin/bash
#
# Runs a specified command on all specified Skia GCE instances.
#
# Copyright 2014 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)

source vm_config.sh

function usage() {
  cat << EOF

usage: $0 "pkill -9 -f tools/perf/record_wpr"

The 1st argument is the user that should run the command.
The 2nd argument is the command that should be run on instances.

EOF
}

if [ $# -ne 2 ]; then
  usage
  exit 2
fi

SSH_USER=$1
CMD=$2

echo "About to run $CMD on instances $VM_BOT_COUNT_START ... $VM_BOT_COUNT_END"
go run vm_run_command_on_instances.go --alsologtostderr \
  --user=$SSH_USER --gcompute_cmd="$GCOMPUTE_CMD" \
  --range_start=$VM_BOT_COUNT_START --range_end=$VM_BOT_COUNT_END \
  --vm_name_prefix=$VM_BOT_NAME --verbose \
  --cmd="$CMD"
