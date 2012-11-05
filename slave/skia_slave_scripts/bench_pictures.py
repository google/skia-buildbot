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
  def _BuildDataFile(self, perf_dir, config, mode, threads, rtree=False):
    data_file = '%s_skp_%s_%s' % (
        super(BenchPictures, self)._BuildDataFile(perf_dir), config, mode)
    if threads > 0:
      data_file += '_%dthreads' % threads
    if rtree:
      data_file += '_rtree'
    return data_file

  def _PictureArgs(self, skp_dir, config, mode, threads, rtree=False):
    args = [skp_dir, '--device', config, '--mode', mode]
    if mode == 'tile':
      args.extend([str(self.TILE_X), str(self.TILE_Y)])
    if threads > 0:
      args.extend(['--multi', str(threads)])
    if rtree:
      args.extend(['--bbh', 'rtree'])
    return args

  def _DoBenchPictures(self, config, mode, threads, rtree=False):
    args = self._PictureArgs(skp_dir=self._skp_dir,
                             config=config,
                             mode=mode,
                             threads=threads,
                             rtree=rtree)
    cmd = [self._PathToBinary('bench_pictures')] + args
    if self._perf_data_dir:
      PreBench(self._perf_data_dir)
      cmd += BenchArgs(repeats=self.BENCH_REPEAT_COUNT,
                       data_file=self._BuildDataFile(
                           perf_dir=self._perf_data_dir,
                           config=config,
                           threads=threads))
    misc.Bash(cmd)

  def _Run(self):
    # Run bitmap in tiled mode, in different numbers of threads
    for threads in [0, 2, 4]:
      self._DoBenchPictures(config='bitmap', mode='tile', threads=threads)

    # Maybe run gpu config
    gyp_defines = os.environ.get('GYP_DEFINES', '')
    if ('skia_gpu=0' not in gyp_defines):
      self._DoBenchPictures(config='gpu', mode='tile', threads=0)

    # Run bitmap in record config without rtree
    self._DoBenchPictures(config='bitmap', mode='record', threads=0,
                          rtree=False)
    # Run bitmap in record config with rtree
    self._DoBenchPictures(config='bitmap', mode='record', threads=0, rtree=True)

if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(BenchPictures))
