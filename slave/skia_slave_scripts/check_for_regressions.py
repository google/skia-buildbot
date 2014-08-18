#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Check for regressions in bench data. """

from build_step import BuildStep

import sys


class CheckForRegressions(BuildStep):
  def _Run(self):
    return  # TODO(mtklein): delete after master restart


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(CheckForRegressions))
