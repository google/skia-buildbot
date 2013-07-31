#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Clean step """

from build_step import BuildStep
import sys


class Clean(BuildStep):
  def _Run(self):
    self._flavor_utils.MakeClean()


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(Clean))