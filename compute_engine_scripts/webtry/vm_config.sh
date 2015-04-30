#!/bin/bash
#
# This file contains constants for the shell scripts which interact
# with the skia-webtry Google Compute Engine instance.
#
# Copyright 2014 Google Inc. All Rights Reserved.

# Sets all constants in compute_engine_cfg.py as env variables.
$(python ../compute_engine_cfg.py)
if [ $? != "0" ]; then
  echo "Failed to read compute_engine_cfg.py!"
  exit 1
fi

ZONE=us-central1-f

# The base names of the VM instances. Actual names are VM_NAME_BASE-name-zone
VM_NAME_BASE=${VM_NAME_BASE:="skia"}

# The name of instance where skfiddle.com is running.
INSTANCE_NAME=${VM_NAME_BASE}-webtry
TEST_INSTANCE_NAME=${VM_NAME_BASE}-webtry-test

WEBTRY_IP_ADDRESS=104.154.112.255
WEBTRY_MACHINE_TYPE=n1-highmem-8
WEBTRY_IMAGE=ubuntu-1410-utopic-v20150318c
