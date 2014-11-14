#!/bin/bash
#
# Runs a specified command on all specified chromecompute instances.
#
# Copyright 2014 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)

source vm_config.sh

function usage() {
  cat << EOF

usage: $0 "pkill -9 -f tools/perf/record_wpr"

The first argument is the command that should be run on instances.

EOF
}

if [ $# -ne 1 ]; then
  usage
  exit 1
fi

CMD=$1

echo "About to run $CMD on instances..."
for MACHINE_IP in $(seq $VM_BOT_COUNT_START $VM_BOT_COUNT_END); do
  INSTANCE_NAME=${VM_BOT_NAME}-`printf "%03d" ${MACHINE_IP}`
  echo "========== $INSTANCE_NAME =========="
  $GCOMPUTE_CMD ssh --ssh_user=$PROJECT_USER $INSTANCE_NAME "$CMD"
  echo "===================================="
done

