#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia bench executable. """

from build_step import BuildStep, BuildStepWarning
from chromeos_build_step import ChromeOSBuildStep
from run_bench import RunBench
import sys


class ChromeOSRunBench(ChromeOSBuildStep, RunBench):
  def _Run(self):
    # TODO(borenet): Re-enable this step once the crash is fixed.
    # RunBench._Run(self)
    raise BuildStepWarning('Skipping bench on ChromeOS until crash is fixed.')


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(ChromeOSRunBench))