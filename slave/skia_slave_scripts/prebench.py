#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Prepare runtime resources that are needed by Bench builders but not
    Test builders. """

from utils import misc
from build_step import BuildStep
import sys


class PreBench(BuildStep):
  def _Run(self):
    if self._perf_data_dir:
      # Create the data dir if it doesn't exist.
      self._flavor_utils.CreateCleanDeviceDirectory(self._device_dirs.PerfDir())


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(PreBench))
