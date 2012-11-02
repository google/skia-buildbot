#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Check out the Skia buildbot scripts. """

from utils import misc
from build_step import BuildStep
import os
import sys


BUILD_DIR_DEPTH = 6


class UpdateScripts(BuildStep):
  def _Run(self):
    buildbot_dir = os.path.join(*[os.pardir for i in range(BUILD_DIR_DEPTH)])
    os.chdir(buildbot_dir)
    if os.name == 'nt':
      gclient = 'gclient.bat'
    else:
      gclient = 'gclient'

    misc.Bash([gclient, 'sync'])


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(UpdateScripts))