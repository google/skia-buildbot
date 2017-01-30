#!/bin/bash
#
# This file contains constants for the shell scripts which interact
# with the skia-status Google Compute Engine instance.
#
# Copyright 2015 Google Inc. All Rights Reserved.

# Sets all constants in compute_engine_cfg.py as env variables.

$(python ../compute_engine_cfg.py)
if [ $? != "0" ]; then
  echo "Failed to read compute_engine_cfg.py!"
  exit 1
fi

VM_ID=${VM_ID:-prod}
case "$VM_ID" in
  prod)
    INSTANCE_NAME=skia-status
    IP_ADDRESS=104.154.112.113
    ;;

  internal)
    INSTANCE_NAME=skia-status-internal
    IP_ADDRESS=104.154.112.138
    ;;

  *)
    echo "Invalid instance name '${VM_ID}'"
    exit 1
    ;;

esac
