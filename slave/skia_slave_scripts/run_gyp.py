#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run GYP to generate project files. """

from build_step import BuildStep
import sys


class RunGYP(BuildStep):
  def _Run(self):
    self._flavor_utils.RunGYP()


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(RunGYP))