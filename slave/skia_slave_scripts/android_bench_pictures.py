#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia bench_pictures executable. """

from android_build_step import AndroidBuildStep
from bench_pictures import BenchPictures
from build_step import BuildStep
from utils import misc
import os
import sys

class AndroidBenchPictures(AndroidBuildStep, BenchPictures):
  def _Run(self, args):
    cmd_args = [self._android_skp_dir]
    # TODO(borenet): Share logic with RunBench, since much is duplicated
    if self._perf_data_dir:
      self._PreBench()
      misc.RunADB(self._serial, ['root'])
      misc.RunADB(self._serial, ['remount'])
      try:
        misc.RunADB(self._serial, ['shell', 'rm', '-r',
                                   self._android_skp_perf_dir])
      except:
        pass
      misc.RunADB(self._serial, ['shell', 'mkdir', '-p',
                                 self._android_skp_perf_dir])
      cmd_args += self._BuildArgs(self.BENCH_REPEAT_COUNT,
                                  self._BuildDataFile(
                                     self._android_skp_perf_dir))
      misc.Run(self._serial, 'bench_pictures', arguments=cmd_args)
      misc.RunADB(self._serial, ['pull',
                                 self._BuildDataFile(
                                     self._android_skp_perf_dir),
                                 self._perf_data_dir])
      misc.RunADB(self._serial, ['shell', 'rm', '-r',
                                 self._android_skp_perf_dir])
    else:
      misc.Run(self._serial, 'bench_pictures', cmd_args)

if '__main__' == __name__:
  sys.exit(BuildStep.Run(AndroidBenchPictures))

