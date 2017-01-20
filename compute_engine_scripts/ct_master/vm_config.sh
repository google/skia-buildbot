#!/bin/bash
#
# This file contains constants for the shell scripts which interact
# with the skia-ct-master Google Compute Engine instance.
#
# Copyright 2017 Google Inc. All Rights Reserved.

# Sets all constants in compute_engine_cfg.py as env variables.
$(python ../compute_engine_cfg.py)
if [ $? != "0" ]; then
  echo "Failed to read compute_engine_cfg.py!"
  exit 1
fi

# The base names of the VM instances. Actual names are VM_NAME_BASE-name
VM_NAME_BASE=${VM_NAME_BASE:="skia"}

# The name of instance where skia ct master is running on.
INSTANCE_NAME=${VM_NAME_BASE}-ct-master

CT_MASTER_IP_ADDRESS=104.154.112.17
CT_MASTER_MACHINE_TYPE=n1-highmem-16
CT_MASTER_SOURCE_SNAPSHOT=skia-systemd-pushable-base
CT_MASTER_SCOPES='https://www.googleapis.com/auth/devstorage.full_control,https://www.googleapis.com/auth/userinfo.email,https://www.googleapis.com/auth/userinfo.profile,https://www.googleapis.com/auth/gerritcodereview,https://www.googleapis.com/auth/pubsub'

