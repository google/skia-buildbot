#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Install the Skia Android APK. """

from android_build_step import AndroidBuildStep
from build_step import BuildStep
from utils import android_utils
import sys


class AndroidInstallAPK(AndroidBuildStep):
  def _Run(self):
    release_mode = self._configuration == 'Release'
    android_utils.Install(self._serial, release_mode,
                          install_launcher=self._has_root)

if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(AndroidInstallAPK))
