#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia bench_pictures executable. """

from android_render_pictures import AndroidRenderPictures
from android_run_bench import DoBench
from bench_pictures import BenchPictures
from build_step import BuildStep
import sys

class AndroidBenchPictures(BenchPictures, AndroidRenderPictures):
  def __init__(self, args, attempts=1, timeout=134400):
    super(AndroidBenchPictures, self).__init__(args, attempts=attempts,
                                               timeout=timeout)

  def _DoBenchPictures(self, args):
    data_file = self._BuildDataFile(self._device_dirs.SKPPerfDir(), args)
    args += [self._device_dirs.SKPDir()]
    DoBench(serial=self._serial,
            executable='bench_pictures',
            perf_data_dir=self._perf_data_dir,
            device_perf_dir=self._device_dirs.SKPPerfDir(),
            data_file=data_file,
            extra_args=args)

  def _Run(self):
    self._PushSKPSources(self._serial)
    super(AndroidBenchPictures, self)._Run()

if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(AndroidBenchPictures))
