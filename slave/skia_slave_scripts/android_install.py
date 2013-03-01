#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Install the Skia Android APK. """

from android_build_step import AndroidBuildStep
from build_step import BuildStep
from install import Install
from utils import android_utils
import glob
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
    for skp in glob.glob(os.path.join(self._skp_dir, '*.skp')):
      android_utils.RunADB(self._serial, ['push', skp,
                                          self._device_dirs.SKPDir()])


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(AndroidInstall))
