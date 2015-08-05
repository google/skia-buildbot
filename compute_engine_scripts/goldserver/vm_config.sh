#!/bin/bash
#
# This file contains constants for the shell scripts which interact
# with the skia-gold Google Compute Engine instance.
#
# Copyright 2014 Google Inc. All Rights Reserved.

set -e

# Sets all constants in compute_engine_cfg.py as env variables.
$(python ../compute_engine_cfg.py)
if [ $? != "0" ]; then
  echo "Failed to read compute_engine_cfg.py!"
  exit 1
fi

# Shared scope that is inherited from compute_engine_cfg.py.
GOLD_SCOPES="$SCOPES"
GOLD_SOURCE_IMAGE="skia-systemd-pushable-base"

case "$1" in
  prod)
    GOLD_MACHINE_TYPE=n1-highmem-16
    GOLD_IP_ADDRESS=104.154.112.104
    GOLD_DATA_DISK_SIZE="2TB"
    ;;

  stage)
    # TODO(stephana): Reduce the instance size once gold is more performant.
    GOLD_MACHINE_TYPE=n1-highmem-16
    GOLD_IP_ADDRESS=104.154.112.105
    GOLD_DATA_DISK_SIZE="500GB"
    ;;

  android)
    GOLD_MACHINE_TYPE=n1-highmem-16
    GOLD_IP_ADDRESS=104.154.112.106
    GOLD_DATA_DISK_SIZE="500GB"
    GOLD_SCOPES="$GOLD_SCOPES,https://www.googleapis.com/auth/androidbuild.internal"
    ;;

  blink)
    GOLD_MACHINE_TYPE=n1-highmem-16
    GOLD_IP_ADDRESS=104.154.112.107
    GOLD_DATA_DISK_SIZE="500GB"
    ;;

  # For testing only. Destroy after creation.
  testinstance)
    GOLD_MACHINE_TYPE=n1-highmem-16
    GOLD_IP_ADDRESS=104.154.112.111
    GOLD_DATA_DISK_SIZE="500GB"
    GOLD_SCOPES="$GOLD_SCOPES,https://www.googleapis.com/auth/androidbuild.internal"
    ;;

  *)
    # There must be a target instance id provided.
    echo "Usage: $0 {prod | stage | android | blink | testinstance}"
    echo "   An instance id must be provided as the first argument."
    exit 1
    ;;

esac

# The base names of the VM instances. Actual names are VM_NAME_BASE-name-zone
VM_NAME_BASE=${VM_NAME_BASE:="skia"}

# The name of instance where gold is running on.
INSTANCE_NAME=${VM_NAME_BASE}-gold-$1
GOLD_DATA_DISK_NAME="$INSTANCE_NAME-data"

# Remove the startup script and generate a new one with the right disk name.
sed "s/GOLD_DATA_DISK_NAME/${GOLD_DATA_DISK_NAME}/g" startup-script.sh.template > startup-script.sh
