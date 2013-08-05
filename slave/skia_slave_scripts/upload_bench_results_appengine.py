#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Upload benchmark results to AppEngine.. """

from build_step import BuildStep
from utils import shell_utils

import os
import skia_vars
import sys


class UploadBenchResultsToAppengine(BuildStep):
  def __init__(self, timeout=1800, no_output_timeout=1200, **kwargs):
    super(UploadBenchResultsToAppengine, self).__init__(
        timeout=timeout,
        no_output_timeout=no_output_timeout,
        **kwargs)

  def _RunInternal(self, representation):
    path_to_bench_graph_svg = os.path.join('bench', 'bench_graph_svg.py')
    graph_title = 'Bench_Performance_for_%s' % self._builder_name

    cmd = ['python', path_to_bench_graph_svg,
           '-d', self._perf_data_dir,
           '-r', '-1',
           '-f', '-1',
           '-l', graph_title,
           '-m', representation,
           '-a', skia_vars.GetGlobalVariable('skia_dashboard_add_point_url'),
           ]
    if os.name == 'nt':
      cmd.extend(['-i', 'c'])  # Ignore cpu time for Windows.

    shell_utils.Bash(cmd)

  def _Run(self):
    if self._perf_data_dir:
      self._RunInternal('25th')


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(UploadBenchResultsToAppengine))
