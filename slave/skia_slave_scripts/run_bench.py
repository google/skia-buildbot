#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia benchmarking executable. """

from build_step import BuildStep
import os
import sys


def BenchArgs(repeats, data_file):
  """ Builds a list containing arguments to pass to bench.

  repeats: integer indicating the number of times to repeat each benchmark
  data_file: filepath to store the log output
  """
  return ['--repeat', '%d' % repeats, '--timers', 'wcg', '--logPerIter',
          '--logFile', data_file]


class RunBench(BuildStep):

  BENCH_REPEAT_COUNT = 20

  def _BuildDataFile(self):
    return os.path.join(self._device_dirs.PerfDir(),
                        'bench_r%s_data' % self._got_revision)

  def _Run(self):
    args = []
    if self._perf_data_dir:
      args.extend(BenchArgs(self.BENCH_REPEAT_COUNT,
                            self._BuildDataFile()))
    self.RunFlavoredCmd('bench', args + self._bench_args)


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(RunBench))
