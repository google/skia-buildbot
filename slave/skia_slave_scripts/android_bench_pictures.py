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
  def _DoBenchPictures(self, config):
    app_data_file = self._BuildDataFile(self._app_dirs.SKPPerfDir(), config)
    adb_data_file = self._BuildDataFile(self._adb_dirs.SKPPerfDir(), config)
    args = self._PictureArgs(self._app_dirs.SKPDir(), config)
    DoBench(serial=self._serial,
            executable='bench_pictures',
            perf_data_dir=self._perf_data_dir,
            adb_perf_dir=self._adb_dirs.SKPPerfDir(),
            app_data_file=app_data_file,
            adb_data_file=adb_data_file,
            extra_args=args)

  def _Run(self, args):
    self._PushSKPSources(self._serial)
    super(AndroidBenchPictures, self)._Run(args)

if '__main__' == __name__:
  sys.exit(BuildStep.Run(AndroidBenchPictures))
