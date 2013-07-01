#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia tests executable. """

from valgrind_build_step import ValgrindBuildStep
from build_step import BuildStep
from run_tests import RunTests
import os
import sys


class ValgrindRunTests(ValgrindBuildStep, RunTests):
  def __init__(self, suppressions_file=os.path.join('tests', 'valgrind.supp'),
               **kwargs):
    super(ValgrindRunTests, self).__init__(suppressions_file=suppressions_file,
                                           **kwargs)


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(ValgrindRunTests))
