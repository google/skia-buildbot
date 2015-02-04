#!/bin/bash
#
# This file contains constants for the shell scripts which interact
# with the skia-webtry-b Google Compute Engine instance.
#
# Copyright 2014 Google Inc. All Rights Reserved.

# Sets all constants in compute_engine_cfg.py as env variables.
$(python ../compute_engine_cfg.py)
if [ $? != "0" ]; then
  echo "Failed to read compute_engine_cfg.py!"
  exit 1
fi

# The base names of the VM instances. Actual names are VM_NAME_BASE-name-zone
VM_NAME_BASE=${VM_NAME_BASE:="skia"}

# The name of instance where skfiddle.com is running on.
INSTANCE_NAME=${VM_NAME_BASE}-webtry-b
TEST_INSTANCE_NAME=${VM_NAME_BASE}-webtry-test

WEBTRY_IP_ADDRESS=108.170.219.69
WEBTRY_MACHINE_TYPE=n1-highmem-8
WEBTRY_IMAGE=backports-debian-7-wheezy-v20140331
