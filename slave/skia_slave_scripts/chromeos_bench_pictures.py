#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia bench_pictures executable. """

from chromeos_render_pictures import ChromeOSRenderPictures
from chromeos_run_bench import DoBench
from bench_pictures import BenchPictures
from build_step import BuildStep
import posixpath
import sys


class ChromeOSBenchPictures(BenchPictures, ChromeOSRenderPictures):
  def _DoBenchPictures(self, args):
    data_file = self._BuildDataFile(self._device_dirs.SKPPerfDir(), args)
    DoBench(executable='skia_bench_pictures',
            perf_data_dir=self._perf_data_dir,
            device_perf_dir=self._device_dirs.SKPPerfDir(),
            data_file=data_file,
            ssh_username=self._ssh_username,
            ssh_host=self._ssh_host,
            ssh_port=self._ssh_port,
            extra_args=args)

  def _Run(self):
    self._PushSKPSources()
    super(ChromeOSBenchPictures, self)._Run()


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(ChromeOSBenchPictures))
