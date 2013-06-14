#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Prepare runtime resources that are needed by Bench builders but not
    Test builders. """

from build_step import BuildStep
import errno
import os
import sys


class PreBench(BuildStep):
  def _Run(self):
    if self._perf_data_dir:
      # Create the perf data dir if it doesn't exist.
      try:
        os.makedirs(self._perf_data_dir)
      except OSError as e:
        if e.errno == errno.EEXIST:
          pass
        else:
          raise e


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(PreBench))
