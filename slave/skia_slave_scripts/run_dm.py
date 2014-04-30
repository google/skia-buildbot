#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Run the Skia DM executable. """

from build_step import BuildStep
import sys


class RunDM(BuildStep):
  def _Run(self):
    # TODO(borenet): --nogpu is only needed for the housekeeper. Remove this
    # flag when DM is running everywhere.
    cmd = ['--nogpu']
    self._flavor_utils.RunFlavoredCmd('dm', cmd)


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(RunDM))
