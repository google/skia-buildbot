#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the cc_unittests executable. """

from build_step import BuildStep
import os
import sys


class CCUnitTests(BuildStep):

  def _Run(self):
    args = []
    if os.name == 'nt':
      # SchedulerTest is flaky, so disabling it for now. For details, see:
      # https://code.google.com/p/chromium/issues/detail?id=380889
      args.append('--gtest_filter=-SchedulerTest.*')
    self._flavor_utils.RunFlavoredCmd('cc_unittests', args)


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(CCUnitTests))
