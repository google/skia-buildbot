#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Install all executables, and any runtime resources that are needed by
    *both* Test and Bench builders. """

from build_step import BuildStep
from utils import gs_utils
import os
import sys


class Install(BuildStep):
  def _Run(self):
    # Push the SKPs to the device.
    skps_need_updating = True
    try:
      # Only push if the existing set is out of date.
      host_timestamp = open(os.path.join(self._skp_dir,
          gs_utils.TIMESTAMP_COMPLETED_FILENAME)).read()
      device_timestamp = self._flavor_utils.ReadFileOnDevice(
          os.path.join(self._device_dirs.SKPDir(),
                       gs_utils.TIMESTAMP_COMPLETED_FILENAME))
      if host_timestamp == device_timestamp:
        print 'SKPs are up to date. Skipping.'
        skps_need_updating = False
      else:
        print 'SKP timestamp does not match:\n%s\n%s\nPushing SKPs...' % (
            device_timestamp, host_timestamp)
    except Exception as e:
      print 'Could not get timestamps: %s' % e
    if skps_need_updating:
      self._flavor_utils.CopyDirectoryContentsToDevice(
          self._skp_dir, self._device_dirs.SKPDir())

    # Push resources to the device.
    self._flavor_utils.CopyDirectoryContentsToDevice(
        self._resource_dir, self._device_dirs.ResourceDir())

    # Initialize a clean scratch directory.
    self._flavor_utils.CreateCleanDeviceDirectory(self._device_dirs.TmpDir())

    # Install the Skia executables.
    self._flavor_utils.Install()

if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(Install))
