#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Calculate and output bench expectations. """

from build_step import BuildStep
from utils import file_utils
from utils import shell_utils

import os
import sys


class GenerateBenchExpectations(BuildStep):
  def __init__(self, timeout=300, no_output_timeout=300, **kwargs):
    super(GenerateBenchExpectations, self).__init__(
        timeout=timeout,
        no_output_timeout=no_output_timeout,
        **kwargs)

  def _RunInternal(self, representation):
    # Generate bench expectations from perf data and output to the given file.
    path_to_gen_bench_expectations = os.path.join('bench',
        'gen_bench_expectations.py')
    expectation_filename = 'bench_expectations_' + self._builder_name + '.txt'
    path_to_expectation_file = os.path.join(self._perf_range_input_dir,
                                            expectation_filename)
    file_utils.create_clean_local_dir(self._perf_range_input_dir)
    cmd = ['python', path_to_gen_bench_expectations,
           '-a', representation,
           '-b', self._builder_name,
           '-d', self._perf_data_dir,
           '-o', path_to_expectation_file,
           '-r', self._got_revision,
          ]

    shell_utils.run(cmd)

  def _Run(self):
    if self._perf_data_dir:
      self._RunInternal('25th')


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(GenerateBenchExpectations))
