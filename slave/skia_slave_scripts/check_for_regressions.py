#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Check for regressions in bench data. """

from build_step import BuildStep
from utils import shell_utils

import os
import sys


class CheckForRegressions(BuildStep):
  def __init__(self, timeout=600, no_output_timeout=600, **kwargs):
    super(CheckForRegressions, self).__init__(
        timeout=timeout,
        no_output_timeout=no_output_timeout,
        **kwargs)

  def _RunInternal(self, representation):
    path_to_bench_graph_svg = os.path.join('bench', 'bench_graph_svg.py')
    path_to_bench_expectations = os.path.join('bench',
                                              'bench_expectations.txt')
    graph_title = 'Bench_Performance_for_%s' % self._builder_name
    cmd = ['python', path_to_bench_graph_svg,
           '-d', self._perf_data_dir,
           '-e', path_to_bench_expectations,
           '-r', '-1',
           '-f', '-1',
           '-l', graph_title,
           '-m', representation,
           ]
    if self._builder_name.find('_Win') >= 0:
      cmd.extend(['-i', 'c'])  # Ignore cpu time for Windows.

    shell_utils.Bash(cmd)

  def _Run(self):
    if self._perf_data_dir:
      for rep in ['avg', 'min', 'med', '25th']:
        self._RunInternal(rep)


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(CheckForRegressions))
