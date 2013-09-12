#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run GYP to generate project files. """

from build_step import BuildStep
import sys


class RunGYP(BuildStep):
  def __init__(self, timeout=15000, no_output_timeout=10000,
               **kwargs):
    super(RunGYP, self).__init__(timeout=timeout,
                                 no_output_timeout=no_output_timeout,
                                 **kwargs)

  def _Run(self):
    self._flavor_utils.RunGYP()


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(RunGYP))
