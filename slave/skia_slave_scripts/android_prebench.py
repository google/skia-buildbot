#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Step to run before the benchmarking steps. """

from android_build_step import AndroidBuildStep
from build_step import BuildStep
from prebench import PreBench
from utils import android_utils
import sys


class AndroidPreBench(AndroidBuildStep, PreBench):
  def _Run(self):
    super(AndroidPreBench, self)._Run()

    if self._perf_data_dir:
      try:
        android_utils.RunADB(self._serial, ['shell', 'rm', '-r',
                                            self._device_dirs.PerfDir()])
      except Exception:
        pass
      android_utils.RunADB(self._serial, ['shell', 'mkdir', '-p',
                                          self._device_dirs.PerfDir()])


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(AndroidPreBench))