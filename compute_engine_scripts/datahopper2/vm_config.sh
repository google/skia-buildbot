#!/bin/bash
#
# This file contains constants for the shell scripts which interact
# with the skia-datahopper2 Google Compute Engine instance.
#
# Copyright 2015 Google Inc. All Rights Reserved.

# Sets all constants in compute_engine_cfg.py as env variables.

$(python ../compute_engine_cfg.py)
if [ $? != "0" ]; then
  echo "Failed to read compute_engine_cfg.py!"
  exit 1
fi

VM_ID="$1"

# The name of instance where datahopper is running.
case "$VM_ID" in
  prod)
    INSTANCE_NAME=skia-datahopper2
    IP_ADDRESS=104.154.112.122
    ;;
  test1)
    INSTANCE_NAME=skia-datahopper-test1
    IP_ADDRESS=104.154.112.124
    ;;
  test2)
    INSTANCE_NAME=skia-datahopper-test2
    IP_ADDRESS=104.154.112.125
    ;;
  *)
    # Must provide a target instance id.
    echo "Usage: $0 {prod | test1 | test2}"
    echo "   An instance id must be provided as the first argument."
    exit 1
    ;;

esac
