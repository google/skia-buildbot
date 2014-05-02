#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Run the Blink AutoRoll bot for Skia."""


import os
import sys

from build_step import BuildStep
from utils import misc
from utils import shell_utils


# TODO(borenet): Set up an automated account for this:
# https://code.google.com/p/chromium/issues/detail?id=339824
DEPS_ROLL_AUTHOR = 'robertphillips@google.com'


class AutoRoll(BuildStep):
  """BuildStep which runs the Blink AutoRoll bot."""

  def _Run(self):
    auto_roll = os.path.join(misc.BUILDBOT_PATH,
                             'chromium_buildbot_tot', 'scripts', 'tools',
                             'blink_roller', 'auto_roll.py')
    chrome_path = os.path.join(os.pardir, 'src')
    # python auto_roll.py <project> <author> <path to chromium/src>
    cmd = ['python', auto_roll, 'skia', DEPS_ROLL_AUTHOR, chrome_path]
    shell_utils.run(cmd)


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(AutoRoll))
