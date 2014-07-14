#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Run the Skia benchmarking executable."""

import os
import sys

from build_step import BuildStep
from utils import gclient_utils

class RunNanobench(BuildStep):
  """A BuildStep that runs nanobench."""

  def __init__(self, timeout=9600, no_output_timeout=9600, **kwargs):
    super(RunNanobench, self).__init__(timeout=timeout,
                                       no_output_timeout=no_output_timeout,
                                       **kwargs)

  def _JSONPath(self):
    git_timestamp = gclient_utils.GetGitRepoPOSIXTimestamp()
    return os.path.join(
            self._device_dirs.PerfDir(),
            'nanobench_%s_%s.json' % ( self._got_revision, git_timestamp))

  def _Run(self):
    args = ['-i', self._device_dirs.ResourceDir()]
    if self._perf_data_dir:
      args.extend(['--outResultsFile', self._JSONPath()])
    self._flavor_utils.RunFlavoredCmd('nanobench', args)


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(RunNanobench))
