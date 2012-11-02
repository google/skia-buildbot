#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia benchmarking executable. """

from utils import misc
from build_step import BuildStep
import errno
import os
import sys

def BenchArgs(repeats, data_file):
  """ Builds a list containing arguments to pass to bench.

  repeats: integer indicating the number of times to repeat each benchmark
  data_file: filepath to store the log output
  """
  return ['--repeat', '%d' % repeats, '--timers', 'wcg', '--logPerIter',
          '%d' %1, '--logFile', data_file]

def PreBench(perf_data_dir):
  """ Creates perf_data_dir if it doesn't exist.

  perf_data_dir: path to create
  """
  try:
    os.makedirs(perf_data_dir)
  except OSError as e:
    if e.errno == errno.EEXIST:
      pass
    else:
      raise e

class RunBench(BuildStep):

  BENCH_REPEAT_COUNT = 20

  def _BuildDataFile(self, perf_dir):
    return os.path.join(perf_dir, 'bench_r%s_data' % self._got_revision)

  def _Run(self):
    cmd = [self._PathToBinary('bench')]
    if self._perf_data_dir:
      PreBench(self._perf_data_dir)
      cmd += BenchArgs(self.BENCH_REPEAT_COUNT,
                       self._BuildDataFile(self._perf_data_dir))
    misc.Bash(cmd + self._bench_args)

if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(RunBench))
