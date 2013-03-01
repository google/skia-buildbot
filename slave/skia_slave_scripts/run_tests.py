#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia tests executable. """

from build_step import BuildStep
import sys


class RunTests(BuildStep):
  def _Run(self):
    self.RunFlavoredCmd('tests', self._test_args)


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(RunTests))