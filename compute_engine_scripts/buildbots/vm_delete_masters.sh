#!/bin/bash
#
# Delete the master instance.
#
# Copyright 2013 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)

source vm_config.sh

for VM in $VM_MASTER_NAMES; do
  $GCOMPUTE_CMD deleteinstance ${VM_NAME_BASE}-${VM}-${ZONE_TAG}
done
