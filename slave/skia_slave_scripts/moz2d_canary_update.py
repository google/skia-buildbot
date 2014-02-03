#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Check out the Skia sources. """


from build_step import BuildStep
from flavor_utils import moz2d_canary_build_step_utils
from update import Update
from utils import shell_utils

import os
import sys


class Moz2DCanaryUpdate(Update):
  def __init__(self, **kwargs):
    super(Moz2DCanaryUpdate, self).__init__(**kwargs)

  def _Run(self):
    super(Moz2DCanaryUpdate, self)._Run()
    os.chdir(moz2d_canary_build_step_utils.MOZ2D_DIR)
    moz2d_rev = shell_utils.run(['git', 'rev-parse', 'HEAD'],
                                 log_in_real_time=False)
    print 'Moz2D updated to %s' % moz2d_rev


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(Moz2DCanaryUpdate))
