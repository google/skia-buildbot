#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Check for regressions in bench data. """

from build_step import BuildStep
from config_private import AUTOGEN_SVN_BASEURL
from slave import slave_utils
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
    # Reads expectations from skia-autogen svn repo using 'svn cat'.
    expectations_filename = ('bench_expectations_' +
        builder_name_schema.GetWaterfallBot(self.builder_name) + '.txt')
    url = '/'.join([AUTOGEN_SVN_BASEURL, 'bench', expectations_filename])

    svn_binary = slave_utils.SubversionExe()
    try:
      output = shell_utils.run([svn_binary, 'cat', url])
    except shell_utils.CommandFailedException:
      print 'Skip due to missing expectations: %s' % url
      return

    path_to_check_bench_regressions = os.path.join('bench',
        'check_bench_regressions.py')

    # Writes the expectations from svn repo to the local file.
    path_to_bench_expectations = os.path.join(
        self._perf_range_input_dir, expectations_filename)
    os.makedirs(self._perf_range_input_dir)
    with open(path_to_bench_expectations, 'w') as file_handle:
      file_handle.write(output)

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
