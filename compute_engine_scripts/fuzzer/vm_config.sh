#!/bin/bash
#
# This file contains constants for the shell scripts which interact
# with the skia-fuzzer Google Compute Engine instance.
#
# Copyright 2015 Google Inc. All Rights Reserved.

# Sets all constants in compute_engine_cfg.py as env variables.
$(python ../compute_engine_cfg.py)
if [ $? != "0" ]; then
  echo "Failed to read compute_engine_cfg.py!"
  exit 1
fi

# The base names of the VM instances. Actual names are VM_NAME_BASE-name
VM_NAME_BASE=${VM_NAME_BASE:="skia"}

FUZZER_SOURCE_SNAPSHOT=skia-systemd-pushable-base
FUZZER_SCOPES='https://www.googleapis.com/auth/devstorage.read_write'

FUZZER_FE_IP_ADDRESS=104.154.112.170
FUZZER_FE_MACHINE_TYPE=n1-standard-8
# The name of instance where skia fuzzer frontend is running on.
FUZZER_FE_INSTANCE_NAME=${VM_NAME_BASE}-fuzzer-fe

FUZZER_BE_MACHINE_TYPE=n1-standard-32

FUZZER_BE1_IP_ADDRESS=104.154.112.171
FUZZER_BE1_INSTANCE_NAME=${VM_NAME_BASE}-fuzzer-be-1

FUZZER_BE2_IP_ADDRESS=104.154.112.172
FUZZER_BE2_INSTANCE_NAME=${VM_NAME_BASE}-fuzzer-be-2