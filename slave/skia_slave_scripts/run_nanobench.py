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

  def _AnyMatch(self, *args):
    return any(arg in self._builder_name for arg in args)

  def _AllMatch(self, *args):
    return all(arg in self._builder_name for arg in args)

  def _Run(self):
    args = ['-i', self._device_dirs.ResourceDir()]
    if self._perf_data_dir:
      args.extend(['--outResultsFile', self._JSONPath()])

    run_nanobench = True
    match  = []

    # Disable known problems.
    if self._AnyMatch('Win7', 'Android'):
      # Segfaults when run as GPU bench.  Very large texture?
      match.append('~blurroundrect')

    if self._AnyMatch('Win7'):
      # Seems to be getting hung up in here.
      match.append('~gradient')

    if self._AnyMatch('Nexus7'):
      # Crashes in GPU mode.
      match.append('~draw_stroke')

    if match:
      args.append('--match')
      args.extend(match)

    if run_nanobench:
      self._flavor_utils.RunFlavoredCmd('nanobench', args)


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(RunNanobench))
