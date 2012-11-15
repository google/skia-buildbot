#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia tests executable. """

from utils import shell_utils
from build_step import BuildStep
import sys


class RunTests(BuildStep):
  def _Run(self):
    shell_utils.Bash([self._PathToBinary('tests')] + self._test_args)


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(RunTests))