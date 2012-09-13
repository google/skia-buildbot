#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia bench_pictures executable. """

from android_build_step import AndroidBuildStep
from android_run_bench import DoBench
from bench_pictures import BenchPictures
from build_step import BuildStep
from utils import misc
import os
import sys

class AndroidBenchPictures(BenchPictures, AndroidBuildStep):
  def _DoBenchPictures(self, config):
    data_file = self._BuildDataFile(self._android_skp_perf_dir, config)
    DoBench(self._serial, 'bench_pictures', self._perf_data_dir,
            self._android_skp_perf_dir, data_file,
            [self._android_skp_dir, '--device', config])

if '__main__' == __name__:
  sys.exit(BuildStep.Run(AndroidBenchPictures))
