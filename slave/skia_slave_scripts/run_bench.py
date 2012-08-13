#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia benchmarking executable. """

from utils import misc
from build_step import BuildStep
import errno
import os
import shlex
import sys

class RunBench(BuildStep):

  BENCH_REPEAT_COUNT = 20

  def _PreBench(self):
    try:
      os.makedirs(self._perf_data_dir)
    except OSError as e:
      if e.errno == errno.EEXIST:
        pass
      else:
        raise e

  def _BuildDataFile(self, perf_dir):
    return os.path.join(perf_dir, 'bench_r%s_data' % self._revision)
      
  def _BuildArgs(self, repeats, data_file):
    return ['-repeat', '%d' % repeats, '-timers', 'wcg', '-logPerIter', '%d' %1,
            '-logFile', data_file]
  
  def _Run(self, args):
    cmd = [self._PathToBinary('bench')]
    if self._perf_data_dir:
      self._PreBench()
      cmd += self._BuildArgs(self.BENCH_REPEAT_COUNT,
                             self._BuildDataFile(self._perf_data_dir))
      misc.Bash(cmd)
    else:
      misc.Bash(cmd)

if '__main__' == __name__:
  sys.exit(BuildStep.Run(RunBench))
