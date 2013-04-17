#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Install the Skia Android APK. """

from android_build_step import AndroidBuildStep
from build_step import BuildStep
from install import Install
from utils import android_utils
import os
import sys


class AndroidInstall(AndroidBuildStep, Install):
  def _Run(self):
    super(AndroidInstall, self)._Run()

    release_mode = self._configuration == 'Release'
    android_utils.Install(self._serial, release_mode,
                          install_launcher=self._has_root)

    # Push the SKPs to the device.
    try:
      android_utils.RunADB(self._serial, ['shell', 'rm', '-r', '%s' % \
                                          self._device_dirs.SKPDir()])
    except Exception:
      pass
    android_utils.RunADB(self._serial, ['shell', 'mkdir', '-p',
                                        self._device_dirs.SKPDir()])
    # Push each skp individually, since adb doesn't let us use wildcards
    for skp in os.listdir(self._skp_dir):
      android_utils.RunADB(self._serial,
          ['push', os.path.join(self._skp_dir, skp),
           self._device_dirs.SKPDir()])

    # Push GM expectations to the device.
    try:
      android_utils.RunADB(self._serial,
          ['shell', 'rm', '-r', self._device_dirs.GMExpectedDir()])
    except Exception:
      pass
    android_utils.RunADB(self._serial, ['shell', 'mkdir', '-p',
                                        self._device_dirs.GMExpectedDir()])
    # TODO(borenet) Enable expectations once we're using checksums.  It will
    # take too long to push the expected images, but the checksums will be
    # much faster.
    #expectation_list = os.listdir(self._gm_expected_dir)
    #for e in expectation_list:
    #  if os.path.isfile(os.path.join(self._gm_expected_dir, e)):
    #    android_utils.RunADB(self._serial,
    #        ['push', os.path.join(self._gm_expected_dir, e),
    #         self._device_dirs.GMExpectedDir()])

    # Push resources to the device.
    try:
      android_utils.RunADB(self._serial,
          ['shell', 'rm', '-r', self._device_dirs.ResourceDir()])
    except Exception:
      pass
    android_utils.RunADB(self._serial, ['shell', 'mkdir', '-p',
                                        self._device_dirs.ResourceDir()])
    resource_list = os.listdir(self._resource_dir)
    for res in resource_list:
      if os.path.isfile(os.path.join(self._resource_dir, res)):
        android_utils.RunADB(self._serial,
            ['push', os.path.join(self._resource_dir, res),
             self._device_dirs.ResourceDir()])


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(AndroidInstall))
