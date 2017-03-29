#!/bin/bash
#
# This file contains constants for the shell scripts which interact
# with the skia-autoroll Google Compute Engine instance.
#
# Copyright 2015 Google Inc. All Rights Reserved.

# Sets all constants in compute_engine_cfg.py as env variables.

$(python ../compute_engine_cfg.py)
if [ $? != "0" ]; then
  echo "Failed to read compute_engine_cfg.py!"
  exit 1
fi

VM_ID="$1"

# The name of instance where the autoroller is running.
case "$VM_ID" in
  skia)
    INSTANCE_NAME=skia-autoroll
    IP_ADDRESS=104.154.112.12
    ;;
  catapult)
    INSTANCE_NAME=catapult-autoroll
    IP_ADDRESS=104.154.112.121
    ;;
  nacl)
    INSTANCE_NAME=nacl-autoroll
    IP_ADDRESS=104.154.112.123
    ;;
  pdfium)
    INSTANCE_NAME=pdfium-autoroll
    IP_ADDRESS=104.154.123.210
    ;;
  *)
    # Must provide a target instance id.
    echo "Usage: $0 {skia | catapult | nacl | pdfium}"
    echo "   An instance id must be provided as the first argument."
    exit 1
    ;;

esac

MACHINE_TYPE=n1-standard-2
SOURCE_SNAPSHOT=skia-systemd-pushable-base
SCOPES='https://www.googleapis.com/auth/devstorage.full_control https://www.googleapis.com/auth/userinfo.email https://www.googleapis.com/auth/userinfo.profile'

DATA_DISK_NAME="$INSTANCE_NAME-data"

# Remove the startup script and generate a new one with the right disk name.
sed "s/DATA_DISK_NAME/${DATA_DISK_NAME}/g" startup-script.sh.template > startup-script.sh
