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
    serial = misc.GetSerial(self._device)
    # TODO(borenet): Share logic with RunBench, since much is duplicated
    if self._perf_data_dir:
      self._PreBench()
      misc.RunADB(serial, ['root'])
      misc.RunADB(serial, ['remount'])
      try:
        misc.RunADB(serial, ['shell', 'rm', '-r', self._android_skp_perf_dir])
      except:
        pass
      misc.RunADB(serial, ['shell', 'mkdir', '-p', self._android_skp_perf_dir])
      cmd_args = [self._android_skp_dir]
      cmd_args += self._BuildArgs(self.BENCH_REPEAT_COUNT,
                                 self._BuildDataFile(self._android_skp_perf_dir))
      misc.Run(serial, 'bench_pictures', arguments=cmd_args)
      misc.RunADB(serial, ['pull',
                           self._BuildDataFile(self._android_skp_perf_dir),
                           self._perf_data_dir])
      misc.RunADB(serial, ['shell', 'rm', '-r', self._android_skp_perf_dir])
    else:
      misc.Run(serial, 'bench_pictures', self._android_skp_dir)

if '__main__' == __name__:
  sys.exit(BuildStep.Run(AndroidBenchPictures))

