#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia bench_pictures executable. """

from utils import shell_utils
from build_step import BuildStep
from run_bench import BenchArgs
from run_bench import RunBench
from run_bench import PreBench
import os
import sys


# Skipping these for now to avoid excessively long cycle times.
RUNNING_ALL_CONFIGURATIONS = False


class BenchPictures(RunBench):
  def __init__(self, args, attempts=1, timeout=16800):
    super(BenchPictures, self).__init__(args, attempts=attempts,
                                        timeout=timeout)

  def _BuildDataFile(self, perf_dir, args):
    data_file = '%s_skp_%s' % (
        super(BenchPictures, self)._BuildDataFile(perf_dir),
        '_'.join(args).replace('-', '').replace(':', '-'))
    return data_file

  def _GetSkpDir(self):
    return self._skp_dir

  def _GetPerfDataDir(self):
    return self._perf_data_dir

  def _PopulateSkpDir(self):
    # The skp dir comes from skia repository, nothing to do here.
    pass

  def _DoBenchPictures(self, args):
    cmd = [self._PathToBinary('bench_pictures'), self._GetSkpDir()] + args
    if self._GetPerfDataDir():
      PreBench(self._GetPerfDataDir())
      cmd += BenchArgs(repeats=self.BENCH_REPEAT_COUNT,
                       data_file=self._BuildDataFile(self._GetPerfDataDir(),
                                                     args))
    shell_utils.Bash(cmd)

  def _Run(self):
    self._PopulateSkpDir()
    
    # Default mode: tiled bitmap
    self._DoBenchPictures(['--device', 'bitmap',
                           '--mode', 'tile', str(self.TILE_X),
                                             str(self.TILE_Y)])

    if self._configuration == 'Debug':
      return

    # Run bitmap in tiled mode, in different numbers of threads
    for threads in [2, 3, 4]:
      self._DoBenchPictures(['--device', 'bitmap',
                             '--mode', 'tile', str(self.TILE_X),
                                               str(self.TILE_Y),
                             '--multi', str(threads)])

    # Maybe run gpu config
    gyp_defines = os.environ.get('GYP_DEFINES', '')
    if ('skia_gpu=0' not in gyp_defines and \
        (not hasattr(self, '_device') or self._device != 'nexus_s')):
      self._DoBenchPictures(['--device', 'gpu',
                             '--mode', 'tile', str(self.TILE_X),
                                               str(self.TILE_Y)])

    # copyTiles mode
    self._DoBenchPictures(['--device', 'bitmap',
                           '--mode', 'copyTile', str(self.TILE_X),
                                                 str(self.TILE_Y)])

    # Record mode
    self._DoBenchPictures(['--device', 'bitmap',
                           '--mode', 'record'])

    # Run with different bounding box heirarchies
    configs_to_run_bbh = ['tile', 'record']
    # Don't run playbackCreation on Android
    if not hasattr(self, '_device'): 
      configs_to_run_bbh.append('playbackCreation')
    for mode in configs_to_run_bbh:
      self._DoBenchPictures(['--device', 'bitmap',
                             '--mode', mode] +
                            ([str(self.TILE_X), str(self.TILE_Y)] \
                             if mode == 'tile' else []) +
                            ['--bbh', 'rtree'])
      self._DoBenchPictures(['--device', 'bitmap',
                             '--mode', mode] +
                            ([str(self.TILE_X), str(self.TILE_Y)] \
                             if mode == 'tile' else []) +
                            ['--bbh', 'grid', str(self.TILE_X),
                                              str(self.TILE_Y)])

    # Run with alternate tile sizes
    for tile_x, tile_y in [(512, 512), (1024, 256), (1024, 64)]:
      self._DoBenchPictures(['--device', 'bitmap',
                             '--mode', 'tile', str(tile_x),
                                               str(tile_y)])

    # Run through a set of filters
    if RUNNING_ALL_CONFIGURATIONS:
      filter_types = ['paint', 'point', 'line', 'bitmap', 'rect', 'path', 'text',
                      'all']
      filter_flags = ['antiAlias', 'filterBitmap', 'dither', 'underlineText',
                      'strikeThruText', 'fakeBoldText', 'linearText',
                      'subpixelText', 'devKernText', 'LCDRenderText',
                      'embeddedBitmapText', 'autoHinting', 'verticalText',
                      'genA8FromLCD', 'blur', 'lowBlur', 'hinting',
                      'slightHinting', 'AAClip']
      for filter_type in filter_types:
        for filter_flag in filter_flags:
          self._DoBenchPictures(['--device', 'bitmap',
                                 '--mode', 'tile', str(self.TILE_X),
                                                   str(self.TILE_Y),
                                 '--filter', '%s:%s' % (filter_type,
                                                        filter_flag)])

if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(BenchPictures))
