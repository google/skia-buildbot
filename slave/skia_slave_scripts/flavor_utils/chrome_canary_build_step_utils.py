#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Utilities for Chrome canary build steps. """

from default_build_step_utils import DefaultBuildStepUtils
from common import chromium_utils
from utils import shell_utils

import os


class ChromeCanaryBuildStepUtils(DefaultBuildStepUtils):
  def Compile(self, target):
    if 'VS2012' in self._step.builder_name:
      os.environ['GYP_MSVS_VERSION'] = '2012'
    os.environ['GYP_DEFINES'] = self._step.args['gyp_defines']
    print 'GYP_DEFINES="%s"' % os.environ['GYP_DEFINES']
    make_cmd = 'ninja'
    cmd = [make_cmd,
           '-C', os.path.join('out', self._step.configuration),
           target,
           ]
    cmd.extend(self._step.make_flags)
    shell_utils.Bash(cmd)

  def MakeClean(self):
    chromium_utils.RemoveDirectory('out')

  def RunGYP(self):
    os.environ['GYP_DEFINES'] = self._step.args['gyp_defines']
    print 'GYP_DEFINES="%s"' % os.environ['GYP_DEFINES']
    python = 'python.bat' if os.name == 'nt' else 'python'
    shell_utils.Bash([python, os.path.join('build', 'gyp_chromium')])
