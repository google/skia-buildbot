#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Run the Skia DM executable. """

from build_step import BuildStep
import sys


class RunDM(BuildStep):
  def _Run(self):
    args = ['-v']
    if 'Housekeeper' in self._builder_name:
      args.append('--nogpu')

    self._flavor_utils.RunFlavoredCmd('dm', args)


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(RunDM))
