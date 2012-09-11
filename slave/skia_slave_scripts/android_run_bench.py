#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia bench executable. """

from android_build_step import AndroidBuildStep
from build_step import BuildStep
from run_bench import RunBench
from utils import misc
import sys

ANDROID_PERF_DIR = '/sdcard/skia_perf'

class AndroidRunBench(AndroidBuildStep, RunBench):
  def _Run(self, args):
    if self._perf_data_dir:
      self._PreBench()
      misc.RunADB(self._serial, ['root'])
      misc.RunADB(self._serial, ['remount'])
      try:
        misc.RunADB(self._serial, ['shell', 'rm', '-r', ANDROID_PERF_DIR])
      except:
        pass
      misc.RunADB(self._serial, ['shell', 'mkdir', '-p', ANDROID_PERF_DIR])
      misc.Run(self._serial, 'bench', arguments=self._BuildArgs(
          self.BENCH_REPEAT_COUNT, self._BuildDataFile(ANDROID_PERF_DIR)))
      misc.RunADB(self._serial, ['pull',
                                 self._BuildDataFile(ANDROID_PERF_DIR),
                                 self._perf_data_dir])
      misc.RunADB(self._serial, ['shell', 'rm', '-r', ANDROID_PERF_DIR])
    else:
      misc.Run(self._serial, 'bench')

if '__main__' == __name__:
  sys.exit(BuildStep.Run(AndroidRunBench))