#!/bin/bash
#
# Delete the master instance.
#
# Copyright 2013 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)

source vm_config.sh

$GCOMPUTE_CMD deleteinstance ${VM_NAME_BASE}-${VM_MASTER_NAME}-${ZONE_TAG}
