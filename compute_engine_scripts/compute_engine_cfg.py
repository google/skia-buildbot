#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""This file contains config constants for the Skia's GCE instances."""

import os
import types


PROJECT_USER = 'default'
SKIA_NETWORK_NAME = 'default'
SKIA_REPO_DIR = '/home/%s/storage/skia-repo' % PROJECT_USER
SCOPES = 'https://www.googleapis.com/auth/devstorage.full_control,https://www.googleapis.com/auth/userinfo.email,https://www.googleapis.com/auth/pubsub'
SKIA_BOT_LINUX_IMAGE_NAME = 'skia-buildbot-v8'
SKIA_SWARMING_IMAGE_NAME = 'skia-swarming-v3'
SKIA_BOT_WIN_IMAGE_NAME = 'projects/google.com:windows-internal/global/images/windows-server-2008-r2-ent-internal-v20150310'
SKIA_BOT_MACHINE_TYPE = os.environ.get(
    'SKIA_BOT_MACHINE_TYPE', 'n1-standard-16')
# Options are Linux and Windows.
VM_INSTANCE_OS = os.environ.get('VM_INSTANCE_OS', 'Linux')
IP_ADDRESS_WITHOUT_MACHINE_PART = '104.154.112'
VM_BOT_NAME = 'skia-vm'
PERSISTENT_DISK_NAME = 'skia-disk'
VM_PERSISTENT_DISK_SIZE_GB = os.environ.get('VM_PERSISTENT_DISK_SIZE_GB', 300)
# If this is true then the VM instances will automatically try to connect to the
# buildbot master.
VM_IS_BUILDBOT = os.environ.get('VM_IS_BUILDBOT', 0)
# If this is true then the swarming image is used.
VM_IS_SWARMINGBOT = os.environ.get('VM_IS_SWARMINGBOT', 1)
# If this is true then a CT VM is created in the SkiaCT pool.
# These bots are used by the Skia CT builders. Eg: CT_DM_100k_SKPs.
VM_IS_SKIA_CTBOT = os.environ.get('VM_IS_SKIA_CTBOT', 0)
# If this is true then a CT VM is created in the CT pool.
# These bots are used as workers by the CT framework.
VM_IS_CTBOT = os.environ.get('VM_IS_CTBOT', 0)

# The Project ID is found in the Compute tab of the dev console.
# https://console.developers.google.com/project/31977622648
PROJECT_ID = 'google.com:skia-buildbots'

# The (Shared Fate) Zone is conceptually equivalent to a data center cell. VM
# instances live in a zone.
#
# We flip the default one as required by PCRs in bigcluster.
ZONE_TAG = os.environ.get('ZONE_TAG', 'c')

if VM_IS_SKIA_CTBOT:
  # Use skia-ct prefix. Swarming uses this prefix to put them in the 'SkiaCT' pool.
  VM_BOT_NAME = 'skia-ct-vm'
  VM_PERSISTENT_DISK_SIZE_GB = 3000
if VM_IS_CTBOT:
  # Use ct prefix. Swarming uses this prefix to put them in the 'CT' pool.
  VM_BOT_NAME = 'ct-vm'
  PERSISTENT_DISK_NAME = 'ct-disk'
  # Always use us-central1-b for CT bots because of maddigan@'s comment in
  # https://gcpquotatool.googleplex.com/requests?request=ahlzfmdvb2dsZS5jb206Z2NwcXVvdGF0b29sci4LEghDdXN0b21lchiAgICAhN6GCgwLEgxRdW90YVJlcXVlc3QYgICAgIC5hAoM
  ZONE_TAG = 'b'
  IP_ADDRESS_WITHOUT_MACHINE_PART = '104.154.123'

ZONE = 'us-central1-%s' % ZONE_TAG

# The below constants determine which instances the delete and create/setup
# scripts apply to.
# Eg1: VM_BOT_COUNT_START=1 VM_BOT_COUNT_END=5 vm_create_setup_instances.sh
#   The above command will create and setup skia-vm-001 to skia-vm-005.
# Eg2: VM_BOT_COUNT_START=1 VM_BOT_COUNT_END=1 vm_create_setup_instances.sh
#   The above command will create and setup only skia-vm-001.
VM_BOT_COUNT_START = os.environ.get('VM_BOT_COUNT_START', 1)
VM_BOT_COUNT_END = os.environ.get('VM_BOT_COUNT_END', 100)


if __name__ == '__main__':
  # Set all above constants as environment variables if this module is called as
  # a script.
  for var in vars().keys():
    # Ignore if the var is a system var or a module.
    if not var.startswith('__') and not type(vars()[var]) == types.ModuleType:
      print 'export %s=%s' % (var, vars()[var])

