#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Pulls the directory with render results from the Android device. """

from android_build_step import AndroidBuildStep
from build_step import BuildStep
from utils import android_utils
import sys


class AndroidPostRender(AndroidBuildStep):
  def _PullSKPResults(self, serial):
    android_utils.RunADB(serial, ['pull', self._device_dirs.SKPOutDir(),
                                  self._gm_actual_dir])

  def _Run(self):
    self._PullSKPResults(self._serial)

if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(AndroidPostRender))

