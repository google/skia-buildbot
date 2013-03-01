#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Step to run after the benchmarking steps. """

from android_build_step import AndroidBuildStep
from build_step import BuildStep
from postbench import PostBench
from utils import android_utils
import sys


class AndroidPostBench(AndroidBuildStep, PostBench):
  def _Run(self):
    super(AndroidPostBench, self)._Run()

    if self._perf_data_dir:
      android_utils.RunADB(self._serial, ['pull', self._device_dirs.PerfDir(),
                                          self._perf_data_dir])
      android_utils.RunADB(self._serial, ['shell', 'rm', '-r',
                                          self._device_dirs.PerfDir()])


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(AndroidPostBench))