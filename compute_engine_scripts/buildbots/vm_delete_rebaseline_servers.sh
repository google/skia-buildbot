#!/bin/bash
#
# Delete the rebaseline_server instance(s).
#
# Copyright 2014 Google Inc. All Rights Reserved.
# Author: epoger@google.com (Elliot Poger)

source vm_config.sh

for VM in $VM_REBASELINESERVER_NAMES; do
  $GCOMPUTE_CMD deleteinstance ${VM_NAME_BASE}-${VM}-${ZONE_TAG}
done
