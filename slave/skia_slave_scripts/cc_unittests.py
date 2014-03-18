#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the cc_unittests executable. """

from build_step import BuildStep
import sys


class CCUnitTests(BuildStep):

  def _Run(self):
    self._flavor_utils.RunFlavoredCmd('cc_unittests', [])


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(CCUnitTests))
