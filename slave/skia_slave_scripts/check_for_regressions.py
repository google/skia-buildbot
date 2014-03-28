#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Check for regressions in bench data. """

from build_step import BuildStep
from utils import shell_utils

import builder_name_schema
import os
import sys


class CheckForRegressions(BuildStep):
  def __init__(self, timeout=600, no_output_timeout=600, **kwargs):
    super(CheckForRegressions, self).__init__(
        timeout=timeout,
        no_output_timeout=no_output_timeout,
        **kwargs)

  def _RunInternal(self, representation):
    path_to_check_bench_regressions = os.path.join('bench',
        'check_bench_regressions.py')
    path_to_bench_expectations = os.path.join(
        'expectations',
        'bench',
        'bench_expectations_%s.txt' % builder_name_schema.GetWaterfallBot(
            self._builder_name))
    if not os.path.isfile(path_to_bench_expectations):
      print 'Skip due to missing expectations: %s' % path_to_bench_expectations
      return
    cmd = ['python', path_to_check_bench_regressions,
           '-a', representation,
           '-b', self._builder_name,
           '-d', self._perf_data_dir,
           '-e', path_to_bench_expectations,
           '-r', self._got_revision,
           ]

    shell_utils.run(cmd)

  def _Run(self):
    if self._perf_data_dir:
      self._RunInternal('25th')


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(CheckForRegressions))
