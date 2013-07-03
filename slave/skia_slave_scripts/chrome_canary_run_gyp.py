#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run GYP to generate project files. """

from build_step import BuildStep
from utils import shell_utils
import os
import sys


class ChromeCanaryRunGYP(BuildStep):
  def _Run(self):
    os.environ['GYP_DEFINES'] = self._args['gyp_defines']
    print 'GYP_DEFINES="%s"' % os.environ['GYP_DEFINES']
    python = 'python.bat' if os.name == 'nt' else 'python'
    shell_utils.Bash([python, os.path.join('build', 'gyp_chromium')])


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(ChromeCanaryRunGYP))