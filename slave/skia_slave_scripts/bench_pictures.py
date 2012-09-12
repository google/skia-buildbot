#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia bench_pictures executable. """

from utils import misc
from build_step import BuildStep
from run_bench import RunBench
import os
import sys

class BenchPictures(RunBench):
  def _BuildDataFile(self, perf_dir):
    return '%s_skp' % super(BenchPictures, self)._BuildDataFile(perf_dir)

  def _Run(self, args):
    cmd = [self._PathToBinary('bench_pictures'), self._skp_dir]
    if self._perf_data_dir:
      self._PreBench()
      cmd += self._BuildArgs(self.BENCH_REPEAT_COUNT,
                             self._BuildDataFile(self._perf_data_dir))
    misc.Bash(cmd)

if '__main__' == __name__:
  sys.exit(BuildStep.Run(BenchPictures))

