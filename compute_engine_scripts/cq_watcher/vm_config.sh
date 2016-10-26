#!/bin/bash
#
# This file contains constants for the shell scripts which interact
# with the skia-cq-watcher Google Compute Engine instance.
#
# Copyright 2016 Google Inc. All Rights Reserved.

# Sets all constants in compute_engine_cfg.py as env variables.
$(python ../compute_engine_cfg.py)
if [ $? != "0" ]; then
  echo "Failed to read compute_engine_cfg.py!"
  exit 1
fi

# The base names of the VM instances. Actual names are VM_NAME_BASE-name
VM_NAME_BASE=${VM_NAME_BASE:="skia"}

# The name of instance where skia cq watcher is running on.
INSTANCE_NAME=${VM_NAME_BASE}-cq-watcher

CQ_WATCHER_IP_ADDRESS=104.154.112.115
CQ_WATCHER_MACHINE_TYPE=n1-highmem-2
CQ_WATCHER_SOURCE_SNAPSHOT=skia-systemd-pushable-base
CQ_WATCHER_SCOPES='https://www.googleapis.com/auth/devstorage.full_control'

