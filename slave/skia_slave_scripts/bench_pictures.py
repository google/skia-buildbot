#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia bench_pictures executable. """

from utils import misc
from build_step import BuildStep
from run_bench import BenchArgs
from run_bench import RunBench
from run_bench import PreBench
import os
import sys

class BenchPictures(RunBench):
  def _BuildDataFile(self, perf_dir, config):
    return '%s_skp_%s' % (super(BenchPictures, self)._BuildDataFile(perf_dir),
                          config)

  def _PictureArgs(self, skp_dir, config, threads):
    args = [skp_dir,
            '--device', config,
            '--mode', 'tile', str(self.TILE_X), str(self.TILE_Y)]
    if threads > 0:
      args.extend(['--multi', str(threads)])
    return args

  def _DoBenchPictures(self, config, threads):
    args = self._PictureArgs(self._skp_dir, config, threads)
    cmd = [self._PathToBinary('bench_pictures')] + args
    if self._perf_data_dir:
      PreBench(self._perf_data_dir)
      cmd += BenchArgs(self.BENCH_REPEAT_COUNT,
                       self._BuildDataFile(self._perf_data_dir, config))
    misc.Bash(cmd)

  def _Run(self):
    # Skipping multithreaded bench_pictures for now, since it's crashing
    #for threads in [0, 2, 4]:
    #  self._DoBenchPictures('bitmap', threads)
    self._DoBenchPictures('bitmap', 0)

    gyp_defines = os.environ.get('GYP_DEFINES', '')
    if ('skia_gpu=0' not in gyp_defines):
      self._DoBenchPictures('gpu', 0)


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(BenchPictures))
