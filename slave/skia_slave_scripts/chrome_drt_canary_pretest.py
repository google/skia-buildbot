#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run this before running any tests. """

from build_step import BuildStep
import sys


class ChromeDRTCanaryPreTest(BuildStep):
  def _Run(self):
    self._flavor_utils.CreateCleanHostDirectory(
        self._flavor_utils.baseline_dir)
    self._flavor_utils.CreateCleanHostDirectory(
        self._flavor_utils.result_dir)


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(ChromeDRTCanaryPreTest))
