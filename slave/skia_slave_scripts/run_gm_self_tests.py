#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Run self-tests within gm/ dir. """

import os
import sys

from build_step import BuildStep
from py.utils import shell_utils


class RunGmSelfTests(BuildStep):
  def _Run(self):
    shell_utils.run(os.path.join('gm', 'tests', 'run.sh'))


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(RunGmSelfTests))
