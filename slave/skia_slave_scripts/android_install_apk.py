#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Install the Skia Android APK. """

from android_build_step import AndroidBuildStep
from build_step import BuildStep
from utils import misc
import os
import sys

class AndroidInstallAPK(AndroidBuildStep):
  def _Run(self):
    path_to_apk = os.path.join('out', self._configuration, 'android', 'bin',
                               'SkiaAndroid.apk')
    misc.Install(self._serial, path_to_apk)
    # Also push the skia_launcher executable to the device.
    misc.RunADB(self._serial, ['push', self._PathToBinary('skia_launcher'),
                               '/system/bin/'])

if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(AndroidInstallAPK))
