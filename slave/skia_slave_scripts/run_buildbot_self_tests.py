#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Run self-tests within buildbot code. """

import sys

from build_step import BuildStep
from utils import misc
from utils import shell_utils


class BuildbotSelfTests(BuildStep):
  def _Run(self):
    with misc.ChDir(misc.BUILDBOT_PATH):
      shell_utils.run(['python', 'run_unittests'])


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(BuildbotSelfTests))
